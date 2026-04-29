package httputil_test

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/foodsea/optimization/internal/platform/httputil"
	sherrors "github.com/foodsea/optimization/internal/shared/errors"
)

func init() { gin.SetMode(gin.TestMode) }

func doRequest(handler gin.HandlerFunc) *httptest.ResponseRecorder {
	r := gin.New()
	r.GET("/", handler)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/", http.NoBody))
	return w
}

func decodeResponse(t *testing.T, w *httptest.ResponseRecorder) httputil.Response {
	t.Helper()
	var resp httputil.Response
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	return resp
}

func TestOK(t *testing.T) {
	w := doRequest(func(c *gin.Context) { httputil.OK(c, gin.H{"key": "val"}) })
	assert.Equal(t, http.StatusOK, w.Code)
	resp := decodeResponse(t, w)
	assert.NotNil(t, resp.Data)
	assert.Empty(t, resp.Error)
}

func TestCreated(t *testing.T) {
	w := doRequest(func(c *gin.Context) { httputil.Created(c, gin.H{"id": "1"}) })
	assert.Equal(t, http.StatusCreated, w.Code)
}

func TestNoContent(t *testing.T) {
	w := doRequest(func(c *gin.Context) { httputil.NoContent(c) })
	assert.Equal(t, http.StatusNoContent, w.Code)
}

func TestBadRequest(t *testing.T) {
	w := doRequest(func(c *gin.Context) { httputil.BadRequest(c, "bad input") })
	assert.Equal(t, http.StatusBadRequest, w.Code)
	resp := decodeResponse(t, w)
	assert.Equal(t, "bad input", resp.Error)
}

func TestHandleError_NotFound(t *testing.T) {
	w := doRequest(func(c *gin.Context) { httputil.HandleError(c, sherrors.ErrNotFound) })
	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestHandleError_InvalidInput(t *testing.T) {
	w := doRequest(func(c *gin.Context) { httputil.HandleError(c, sherrors.ErrInvalidInput) })
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestHandleError_AlreadyExists(t *testing.T) {
	w := doRequest(func(c *gin.Context) { httputil.HandleError(c, sherrors.ErrAlreadyExists) })
	assert.Equal(t, http.StatusConflict, w.Code)
}

func TestHandleError_Unauthorized(t *testing.T) {
	w := doRequest(func(c *gin.Context) { httputil.HandleError(c, sherrors.ErrUnauthorized) })
	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestHandleError_Conflict(t *testing.T) {
	w := doRequest(func(c *gin.Context) { httputil.HandleError(c, sherrors.ErrConflict) })
	assert.Equal(t, http.StatusConflict, w.Code)
}

func TestHandleError_ValidationError(t *testing.T) {
	w := doRequest(func(c *gin.Context) {
		httputil.HandleError(c, &sherrors.ValidationError{Field: "email", Message: "invalid"})
	})
	assert.Equal(t, http.StatusUnprocessableEntity, w.Code)
	resp := decodeResponse(t, w)
	assert.Contains(t, resp.Error, "email")
}

func TestHandleError_UnknownError(t *testing.T) {
	w := doRequest(func(c *gin.Context) {
		httputil.HandleError(c, errors.New("some internal error"))
	})
	assert.Equal(t, http.StatusInternalServerError, w.Code)
	resp := decodeResponse(t, w)
	assert.Equal(t, "internal server error", resp.Error)
}

func TestOKWithMeta(t *testing.T) {
	w := doRequest(func(c *gin.Context) {
		httputil.OKWithMeta(c, []string{"a", "b"}, &httputil.Meta{
			Page: 1, PageSize: 20, TotalCount: 2, TotalPages: 1,
		})
	})
	assert.Equal(t, http.StatusOK, w.Code)
	resp := decodeResponse(t, w)
	assert.NotNil(t, resp.Meta)
}
