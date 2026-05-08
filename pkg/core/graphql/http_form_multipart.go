package graphql

import (
	"encoding/json"
	"fmt"
	"io"
	"mime"
	"net/http"
	"os"

	"github.com/99designs/gqlgen/graphql"
	"github.com/99designs/gqlgen/graphql/errcode"
	"github.com/hovanhoa/llmgateway/pkg/core/errors"
	"github.com/vektah/gqlparser/v2/gqlerror"
)

// MultipartForm the Multipart request spec https://github.com/jaydenseric/graphql-multipart-request-spec
// NOTE: This is a copy of the MultipartForm transport from gqlgen, with the error message modified to be more user-friendly.
type MultipartForm struct {
	// MaxUploadSize sets the maximum number of bytes used to parse a request body
	// as multipart/form-data.
	MaxUploadSize int64

	// MaxMemory defines the maximum number of bytes used to parse a request body
	// as multipart/form-data in memory, with the remainder stored on disk in
	// temporary files.
	MaxMemory int64

	// Map of all headers that are added to graphql response. If not
	// set, only one header: Content-Type: application/json will be set.
	ResponseHeaders map[string][]string
}

var _ graphql.Transport = MultipartForm{}

func (f MultipartForm) Supports(r *http.Request) bool {
	if r.Header.Get("Upgrade") != "" {
		return false
	}

	mediaType, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil {
		return false
	}

	return r.Method == "POST" && mediaType == "multipart/form-data"
}

func (f MultipartForm) maxUploadSize() int64 {
	if f.MaxUploadSize == 0 {
		return 32 << 20
	}
	return f.MaxUploadSize
}

func (f MultipartForm) maxMemory() int64 {
	if f.MaxMemory == 0 {
		return 32 << 20
	}
	return f.MaxMemory
}

func (f MultipartForm) Do(w http.ResponseWriter, r *http.Request, exec graphql.GraphExecutor) {
	writeHeaders(w, f.ResponseHeaders)

	start := graphql.Now()

	var err error
	if r.ContentLength > f.maxUploadSize() {
		// NOTE: This is the only line that is modified from the original MultipartForm transport.
		// The original error message is "failed to parse multipart form, request body too large"
		// This was changed to be more user-friendly.
		writeJsonError(w, "The total size of all files exceeds the 256 MB limit")
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, f.maxUploadSize())
	defer func() { _ = r.Body.Close() }()

	mr, err := r.MultipartReader()
	if err != nil {
		w.WriteHeader(http.StatusUnprocessableEntity)
		writeJsonError(w, "failed to parse multipart form")
		return
	}

	part, err := mr.NextPart()
	if err != nil || part.FormName() != "operations" {
		w.WriteHeader(http.StatusUnprocessableEntity)
		writeJsonError(w, "first part must be operations")
		return
	}

	var params graphql.RawParams
	if err = jsonDecode(part, &params); err != nil {
		w.WriteHeader(http.StatusUnprocessableEntity)
		writeJsonError(w, "operations form field could not be decoded")
		return
	}

	part, err = mr.NextPart()
	if err != nil || part.FormName() != "map" {
		w.WriteHeader(http.StatusUnprocessableEntity)
		writeJsonError(w, "second part must be map")
		return
	}

	uploadsMap := map[string][]string{}
	if err = json.NewDecoder(part).Decode(&uploadsMap); err != nil {
		w.WriteHeader(http.StatusUnprocessableEntity)
		writeJsonError(w, "map form field could not be decoded")
		return
	}

	for {
		part, err = mr.NextPart()
		if err == io.EOF {
			break
		} else if err != nil {
			w.WriteHeader(http.StatusUnprocessableEntity)
			writeJsonErrorf(w, "failed to parse part")
			return
		}

		key := part.FormName()
		filename := part.FileName()
		contentType := part.Header.Get("Content-Type")

		paths := uploadsMap[key]
		if len(paths) == 0 {
			w.WriteHeader(http.StatusUnprocessableEntity)
			writeJsonErrorf(w, "invalid empty operations paths list for key %s", key)
			return
		}
		delete(uploadsMap, key)

		var upload graphql.Upload
		if r.ContentLength < f.maxMemory() {
			fileBytes, err := io.ReadAll(part)
			if err != nil {
				w.WriteHeader(http.StatusUnprocessableEntity)
				writeJsonErrorf(w, "failed to read file for key %s", key)
				return
			}
			for _, path := range paths {
				upload = graphql.Upload{
					File:        &bytesReader{s: &fileBytes, i: 0},
					Size:        int64(len(fileBytes)),
					Filename:    filename,
					ContentType: contentType,
				}

				if err := params.AddUpload(upload, key, path); err != nil {
					w.WriteHeader(http.StatusUnprocessableEntity)
					writeJsonGraphqlError(w, err)
					return
				}
			}
		} else {
			tmpFile, err := os.CreateTemp(os.TempDir(), "gqlgen-")
			if err != nil {
				w.WriteHeader(http.StatusUnprocessableEntity)
				writeJsonErrorf(w, "failed to create temp file for key %s", key)
				return
			}
			tmpName := tmpFile.Name()
			defer func() {
				_ = os.Remove(tmpName)
			}()
			fileSize, err := io.Copy(tmpFile, part)
			if err != nil {
				w.WriteHeader(http.StatusUnprocessableEntity)
				if err := tmpFile.Close(); err != nil {
					writeJsonErrorf(w, "failed to copy to temp file and close temp file for key %s", key)
					return
				}
				writeJsonErrorf(w, "failed to copy to temp file for key %s", key)
				return
			}
			if err := tmpFile.Close(); err != nil {
				w.WriteHeader(http.StatusUnprocessableEntity)
				writeJsonErrorf(w, "failed to close temp file for key %s", key)
				return
			}
			for _, path := range paths {
				pathTmpFile, err := os.Open(tmpName)
				if err != nil {
					w.WriteHeader(http.StatusUnprocessableEntity)
					writeJsonErrorf(w, "failed to open temp file for key %s", key)
					return
				}
				defer func() { _ = pathTmpFile.Close() }()
				upload = graphql.Upload{
					File:        pathTmpFile,
					Size:        fileSize,
					Filename:    filename,
					ContentType: contentType,
				}

				if err := params.AddUpload(upload, key, path); err != nil {
					w.WriteHeader(http.StatusUnprocessableEntity)
					writeJsonGraphqlError(w, err)
					return
				}
			}
		}
	}

	for key := range uploadsMap {
		w.WriteHeader(http.StatusUnprocessableEntity)
		writeJsonErrorf(w, "failed to get key %s from form", key)
		return
	}

	params.Headers = r.Header

	params.ReadTime = graphql.TraceTiming{
		Start: start,
		End:   graphql.Now(),
	}

	rc, gerr := exec.CreateOperationContext(r.Context(), &params)
	if gerr != nil {
		resp := exec.DispatchError(graphql.WithOperationContext(r.Context(), rc), gerr)
		w.WriteHeader(statusFor(gerr))
		writeJson(w, resp)
		return
	}
	responses, ctx := exec.DispatchOperation(r.Context(), rc)
	writeJson(w, responses(ctx))
}

func writeHeaders(w http.ResponseWriter, headers map[string][]string) {
	if len(headers) == 0 {
		headers = map[string][]string{
			// Stay with application/json (not application/graphql-response+json)
			// as it is not an actively supported protocol for now
			"Content-Type": {"application/json"},
		}
	}

	for key, values := range headers {
		for _, value := range values {
			w.Header().Add(key, value)
		}
	}
}

func writeJson(w io.Writer, response *graphql.Response) {
	b, err := json.Marshal(response)
	if err != nil {
		panic(fmt.Errorf("unable to marshal %s: %w", string(response.Data), err))
	}
	_, _ = w.Write(b)
}

func writeJsonError(w io.Writer, msg string) {
	writeJson(w, &graphql.Response{Errors: gqlerror.List{{Message: msg}}})
}

func writeJsonErrorf(w io.Writer, format string, args ...any) {
	writeJson(w, &graphql.Response{Errors: gqlerror.List{{Message: fmt.Sprintf(format, args...)}}})
}

func writeJsonGraphqlError(w io.Writer, err ...*gqlerror.Error) {
	writeJson(w, &graphql.Response{Errors: err})
}

func jsonDecode(r io.Reader, val any) error {
	dec := json.NewDecoder(r)
	dec.UseNumber()
	return dec.Decode(val)
}

func statusFor(errs gqlerror.List) int {
	switch errcode.GetErrorKind(errs) {
	case errcode.KindProtocol:
		return http.StatusUnprocessableEntity
	default:
		return http.StatusOK
	}
}

type bytesReader struct {
	s *[]byte
	i int64 // current reading index
}

func (r *bytesReader) Read(b []byte) (n int, err error) {
	if r.s == nil {
		return 0, errors.New("byte slice pointer is nil")
	}
	if r.i >= int64(len(*r.s)) {
		return 0, io.EOF
	}
	n = copy(b, (*r.s)[r.i:])
	r.i += int64(n)
	return
}

func (r *bytesReader) Seek(offset int64, whence int) (int64, error) {
	if r.s == nil {
		return 0, errors.New("byte slice pointer is nil")
	}
	var abs int64
	switch whence {
	case io.SeekStart:
		abs = offset
	case io.SeekCurrent:
		abs = r.i + offset
	case io.SeekEnd:
		abs = int64(len(*r.s)) + offset
	default:
		return 0, errors.New("invalid whence")
	}
	if abs < 0 {
		return 0, errors.New("negative position")
	}
	r.i = abs
	return abs, nil
}
