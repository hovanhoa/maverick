package memkv_test

import (
	"context"
	"errors"
	"testing"

	"github.com/hovanhoa/llmgateway/pkg/driver"
	"github.com/hovanhoa/llmgateway/pkg/driver/memkv"

	"github.com/stretchr/testify/assert"
)

func TestDatabase(t *testing.T) {
	db := memkv.New()
	assert.NotNil(t, db)

	ctx := context.Background()

	found, data, err := db.Get(ctx, "key")
	assert.False(t, found)
	assert.Empty(t, data)
	assert.NoError(t, err)

	err = db.Set(ctx, "key", "value", 0)
	assert.NoError(t, err)

	found, data, err = db.Get(ctx, "key")
	assert.True(t, found)
	assert.Equal(t, "value", data)
	assert.NoError(t, err)

	err = db.Del(ctx, "key")
	assert.NoError(t, err)

	found, data, err = db.Get(ctx, "key")
	assert.False(t, found)
	assert.Empty(t, data)
	assert.NoError(t, err)
}

func TestGetAndSet(t *testing.T) {
	db := memkv.New()
	assert.NotNil(t, db)

	ctx := context.Background()

	assert.NoError(t, db.Set(ctx, "key1", "value1", 0))
	assert.NoError(t, db.Set(ctx, "key2", "value2", 0))

	err := db.GetAndSet(
		ctx,
		func(kv map[string]string) (newKV map[string]string, err error) {
			return map[string]string{
				"key1": "value_new1",
				"key3": "value_new3",
				"key4": "value_new4",
			}, nil
		},
		"key1",
		"key3",
		"key5",
	)

	assert.NoError(t, err)

	found, data, err := db.Get(ctx, "key1")
	assert.NoError(t, err)
	assert.True(t, found)
	assert.Equal(t, "value_new1", data)

	found, data, err = db.Get(ctx, "key2")
	assert.NoError(t, err)
	assert.True(t, found)
	assert.Equal(t, "value2", data)

	found, data, err = db.Get(ctx, "key3")
	assert.NoError(t, err)
	assert.True(t, found)
	assert.Equal(t, "value_new3", data)

	found, data, err = db.Get(ctx, "key4")
	assert.NoError(t, err)
	assert.False(t, found)
	assert.Equal(t, "", data)

	found, data, err = db.Get(ctx, "key5")
	assert.NoError(t, err)
	assert.False(t, found)
	assert.Equal(t, "", data)
}

func TestGetAndSet_MapperError(t *testing.T) {
	db := memkv.New()
	assert.NotNil(t, db)

	ctx := context.Background()

	assert.NoError(t, db.Set(ctx, "key1", "value1", 0))

	err := db.GetAndSet(
		ctx,
		func(kv map[string]string) (newKV map[string]string, err error) {
			return map[string]string{
				"key1": "value_new1",
			}, errors.New("test error")
		},
		"key1",
	)
	assert.Error(t, err)
	assert.Equal(t, err.Error(), "test error")

	found, data, err := db.Get(ctx, "key1")
	assert.NoError(t, err)
	assert.True(t, found)
	assert.Equal(t, "value1", data)
}

func TestClose(t *testing.T) {
	db := memkv.New()
	assert.NotNil(t, db)
	assert.NoError(t, db.Close())
}

func TestGetDriverName(t *testing.T) {
	db := memkv.New()
	assert.Equal(t, driver.Name("memkv"), db.GetDriverName())
}
