package helper

import (
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type partialPingWriter struct {
	gin.ResponseWriter
}

func (w partialPingWriter) Write(data []byte) (int, error) {
	n, err := w.ResponseWriter.Write(data[:3])
	if err != nil {
		return n, err
	}
	return n, io.ErrShortWrite
}

func TestPingDataAccountsOnlyForKeepaliveBytesActuallyWritten(t *testing.T) {
	for _, partial := range []bool{false, true} {
		c, _ := gin.CreateTestContext(httptest.NewRecorder())
		c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
		if partial {
			c.Writer = partialPingWriter{ResponseWriter: c.Writer}
			require.ErrorIs(t, PingData(c), io.ErrShortWrite)
			assert.Equal(t, 3, common.GetContextKeyInt(c, constant.ContextKeyRelayKeepAliveBytes))
			continue
		}
		require.NoError(t, PingData(c))
		require.NoError(t, PingData(c))
		assert.Equal(t, len(": PING\n\n")*2, common.GetContextKeyInt(c, constant.ContextKeyRelayKeepAliveBytes))
		require.NoError(t, StringData(c, `{"content":"answer"}`))
		assert.Greater(t, c.Writer.Size(), common.GetContextKeyInt(c, constant.ContextKeyRelayKeepAliveBytes))
	}
}
