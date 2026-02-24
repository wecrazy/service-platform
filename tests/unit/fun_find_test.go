package unit

import (
	"service-platform/pkg/fun"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGetFileExtension_Known(t *testing.T) {
	known := []string{"image/jpeg", "image/png", "application/pdf", "text/plain"}
	for _, mime := range known {
		got := fun.GetFileExtension(mime)
		assert.NotEqual(t, ".bin", got, "MIME %s should have a real extension", mime)
		assert.Equal(t, byte('.'), got[0], "extension should start with dot")
	}
}

func TestGetFileExtension_Unknown(t *testing.T) {
	assert.Equal(t, ".bin", fun.GetFileExtension("totally/unknown-mime"))
	assert.Equal(t, ".bin", fun.GetFileExtension(""))
}
