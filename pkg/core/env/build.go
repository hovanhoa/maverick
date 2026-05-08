package env

var (
	BuildCommitHash string = ""
)

func GetBuildCommitHash() string {
	return BuildCommitHash
}
