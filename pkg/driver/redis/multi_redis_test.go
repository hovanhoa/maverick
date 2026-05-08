package redis

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/hovanhoa/llmgateway/pkg/driver"
	"github.com/stretchr/testify/assert"
)

type testRedis struct {
	writeKey string
	writeVal string

	delKey string

	found   bool
	readKey string
	readVal string

	closed bool

	err error
}

func (t *testRedis) Get(ctx context.Context, key string) (found bool, value string, err error) {
	t.readKey = key
	return t.found, t.readVal, t.err
}

func (t *testRedis) Set(ctx context.Context, key string, value string, expiration time.Duration) (err error) {
	t.writeKey = key
	t.writeVal = value
	return t.err
}

func (t *testRedis) Del(ctx context.Context, key string) (err error) {
	t.delKey = key
	return t.err
}

func (t *testRedis) Keys(ctx context.Context, pattern string) ([]string, error) {
	panic("not implemented")
}

func (t *testRedis) GetAndSet(ctx context.Context, fn driver.KVMapper, keys ...string) (err error) {
	panic("not implemented")
}

func (t testRedis) GetDriverName() driver.Name {
	panic("not implemented")
}

func (t testRedis) Dehydrate() *driver.SerializedStore {
	panic("not implemented")
}

func (t *testRedis) Close() (err error) {
	t.closed = true
	return t.err
}

func TestMultiRead(t *testing.T) {
	t.Run("test multiple values", func(t *testing.T) {
		r1 := &testRedis{found: true, readVal: "bar"}
		r2 := &testRedis{found: true, readVal: "baz"}

		db := &MultiDatabase{readers: []driver.KVStore{r1, r2}}
		found, val, err := db.Get(context.Background(), "foo")
		assert.True(t, found)
		assert.Equal(t, "bar", val)
		assert.NoError(t, err)
		assert.Equal(t, "foo", r1.readKey)
		assert.Empty(t, r2.readKey)
	})

	t.Run("test one error", func(t *testing.T) {
		r1 := &testRedis{err: errors.New("r1 error")}
		r2 := &testRedis{found: true, readVal: "baz"}

		db := &MultiDatabase{readers: []driver.KVStore{r1, r2}}
		found, val, err := db.Get(context.Background(), "foo")
		assert.True(t, found)
		assert.Equal(t, "baz", val)
		assert.NoError(t, err)
		assert.Equal(t, "foo", r1.readKey)
		assert.Equal(t, "foo", r2.readKey)
	})

	t.Run("test all error", func(t *testing.T) {
		r1 := &testRedis{err: errors.New("r1 error")}
		r2 := &testRedis{err: errors.New("r2 error")}

		db := &MultiDatabase{readers: []driver.KVStore{r1, r2}}
		found, val, err := db.Get(context.Background(), "foo")
		assert.False(t, found)
		assert.Empty(t, val)
		assert.ErrorContains(t, err, "r2 error")
		assert.Equal(t, "foo", r1.readKey)
		assert.Equal(t, "foo", r2.readKey)
	})

	t.Run("test first found", func(t *testing.T) {
		r1 := &testRedis{found: true, readVal: "bar"}
		r2 := &testRedis{found: false}

		db := &MultiDatabase{readers: []driver.KVStore{r1, r2}}
		found, val, err := db.Get(context.Background(), "foo")
		assert.True(t, found)
		assert.Equal(t, "bar", val)
		assert.NoError(t, err)
		assert.Equal(t, "foo", r1.readKey)
		assert.Empty(t, r2.readKey)
	})

	t.Run("test last found", func(t *testing.T) {
		r1 := &testRedis{found: false}
		r2 := &testRedis{found: true, readVal: "bar"}

		db := &MultiDatabase{readers: []driver.KVStore{r1, r2}}
		found, val, err := db.Get(context.Background(), "foo")
		assert.True(t, found)
		assert.Equal(t, "bar", val)
		assert.NoError(t, err)
		assert.Equal(t, "foo", r1.readKey)
		assert.Equal(t, "foo", r2.readKey)
	})

	t.Run("test none found", func(t *testing.T) {
		r1 := &testRedis{found: false}
		r2 := &testRedis{found: false}

		db := &MultiDatabase{readers: []driver.KVStore{r1, r2}}
		found, val, err := db.Get(context.Background(), "foo")
		assert.False(t, found)
		assert.Empty(t, val)
		assert.NoError(t, err)
		assert.Equal(t, "foo", r1.readKey)
		assert.Equal(t, "foo", r2.readKey)
	})
}

func TestMultiWrite(t *testing.T) {
	t.Run("test multiple writers", func(t *testing.T) {
		w1 := &testRedis{}
		w2 := &testRedis{}

		db := &MultiDatabase{writers: []driver.KVStore{w1, w2}}
		err := db.Set(context.Background(), "foo", "bar", 0)
		assert.NoError(t, err)
		assert.Equal(t, "foo", w1.writeKey)
		assert.Equal(t, "foo", w2.writeKey)
		assert.Equal(t, "bar", w1.writeVal)
		assert.Equal(t, "bar", w2.writeVal)
	})

	t.Run("test one error", func(t *testing.T) {
		w1 := &testRedis{err: errors.New("test error")}
		w2 := &testRedis{}

		db := &MultiDatabase{writers: []driver.KVStore{w1, w2}}
		err := db.Set(context.Background(), "foo", "bar", 0)
		assert.NoError(t, err)
		assert.Equal(t, "foo", w1.writeKey)
		assert.Equal(t, "foo", w2.writeKey)
		assert.Equal(t, "bar", w1.writeVal)
		assert.Equal(t, "bar", w2.writeVal)
	})

	t.Run("test all error", func(t *testing.T) {
		w1 := &testRedis{err: errors.New("test error 1")}
		w2 := &testRedis{err: errors.New("test error 2")}

		db := &MultiDatabase{writers: []driver.KVStore{w1, w2}}
		err := db.Set(context.Background(), "foo", "bar", 0)
		assert.ErrorContains(t, err, "test error 2")
		assert.Equal(t, "foo", w1.writeKey)
		assert.Equal(t, "foo", w2.writeKey)
		assert.Equal(t, "bar", w1.writeVal)
		assert.Equal(t, "bar", w2.writeVal)
	})
}

func TestMultiDelete(t *testing.T) {
	t.Run("test multiple writers", func(t *testing.T) {
		w1 := &testRedis{}
		w2 := &testRedis{}

		db := &MultiDatabase{writers: []driver.KVStore{w1, w2}}
		err := db.Del(context.Background(), "foo")
		assert.NoError(t, err)
		assert.Equal(t, "foo", w1.delKey)
		assert.Equal(t, "foo", w2.delKey)
	})

	t.Run("test one error", func(t *testing.T) {
		w1 := &testRedis{err: errors.New("test error")}
		w2 := &testRedis{}

		db := &MultiDatabase{writers: []driver.KVStore{w1, w2}}
		err := db.Del(context.Background(), "foo")
		assert.NoError(t, err)
		assert.Equal(t, "foo", w1.delKey)
		assert.Equal(t, "foo", w2.delKey)
	})

	t.Run("test all error", func(t *testing.T) {
		w1 := &testRedis{err: errors.New("test error 1")}
		w2 := &testRedis{err: errors.New("test error 2")}

		db := &MultiDatabase{writers: []driver.KVStore{w1, w2}}
		err := db.Del(context.Background(), "foo")
		assert.ErrorContains(t, err, "test error 2")
		assert.Equal(t, "foo", w1.delKey)
		assert.Equal(t, "foo", w2.delKey)
	})
}

func TestMultiClose(t *testing.T) {
	t.Run("test multiple close", func(t *testing.T) {
		r1 := &testRedis{}
		w1 := &testRedis{}
		w2 := &testRedis{}

		db := &MultiDatabase{readers: []driver.KVStore{r1}, writers: []driver.KVStore{w1, w2}}
		assert.NoError(t, db.Close())
		assert.True(t, r1.closed)
		assert.True(t, w1.closed)
		assert.True(t, w2.closed)
	})

	t.Run("test one error", func(t *testing.T) {
		r1 := &testRedis{}
		w1 := &testRedis{err: errors.New("test error")}
		w2 := &testRedis{}

		db := &MultiDatabase{readers: []driver.KVStore{r1}, writers: []driver.KVStore{w1, w2}}
		assert.ErrorContains(t, db.Close(), "test error")
		assert.True(t, r1.closed)
		assert.True(t, w1.closed)
		assert.True(t, w2.closed)
	})

	t.Run("test all error", func(t *testing.T) {
		r1 := &testRedis{err: errors.New("test error 1")}
		w1 := &testRedis{err: errors.New("test error 2")}
		w2 := &testRedis{err: errors.New("test error 3")}

		db := &MultiDatabase{readers: []driver.KVStore{r1}, writers: []driver.KVStore{w1, w2}}
		assert.ErrorContains(t, db.Close(), "test error 3")
		assert.True(t, r1.closed)
		assert.True(t, w1.closed)
		assert.True(t, w2.closed)
	})
}

func TestHydrateAndDeHydrate(t *testing.T) {
	db := &MultiDatabase{
		configs: []MultiConfig{
			{
				EnableRead: true,
				Config: Config{
					Host: "host1",
					Port: "1000",
					Pass: "pass1",
				},
			},
			{
				EnableRead:  true,
				EnableWrite: true,
				Config: Config{
					Host: "host2",
					Port: "2000",
					Pass: "pass2",
				},
			},
		},
	}

	sdb := db.Dehydrate()
	assert.NotNil(t, sdb)

	configs := DeserializeMultiConfig(sdb.Config)
	assert.ElementsMatch(t, db.configs, configs)
}
