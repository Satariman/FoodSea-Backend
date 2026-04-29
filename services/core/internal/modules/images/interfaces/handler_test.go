package interfaces_test

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/foodsea/core/internal/modules/images/interfaces"
	"github.com/foodsea/core/internal/platform/httputil"
	sherrors "github.com/foodsea/core/internal/shared/errors"
)

func init() { gin.SetMode(gin.TestMode) }

// --- mocks ---

type mockUploader struct{ mock.Mock }

func (m *mockUploader) Execute(ctx context.Context, productID uuid.UUID, filename string, reader io.Reader, contentType string) (string, error) {
	args := m.Called(ctx, productID, filename, reader, contentType)
	return args.String(0), args.Error(1)
}

type mockDeleter struct{ mock.Mock }

func (m *mockDeleter) Execute(ctx context.Context, productID uuid.UUID) error {
	args := m.Called(ctx, productID)
	return args.Error(0)
}

// --- helpers ---

func buildRouter(h *interfaces.Handler) *gin.Engine {
	r := gin.New()
	r.POST("/admin/products/:id/image", h.UploadImage)
	r.DELETE("/admin/products/:id/image", h.DeleteImage)
	return r
}

func multipartRequest(productID uuid.UUID, fieldName, fileName string, data []byte, partContentType string) (*http.Request, string) {
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	pw, err := w.CreateFormFile(fieldName, fileName)
	if err != nil {
		panic(err)
	}
	if _, err := pw.Write(data); err != nil {
		panic(err)
	}
	if err := w.Close(); err != nil {
		panic(err)
	}
	req := httptest.NewRequest(http.MethodPost, "/admin/products/"+productID.String()+"/image", &buf)
	req.Header.Set("Content-Type", w.FormDataContentType())
	return req, w.FormDataContentType()
}

// --- tests ---

func TestUploadImage_Success(t *testing.T) {
	uploader := &mockUploader{}
	deleter := &mockDeleter{}

	productID := uuid.New()
	wantURL := "http://localhost:9000/product-images/products/x/y.jpg"

	uploader.On("Execute", mock.Anything, productID, mock.Anything, mock.Anything, mock.Anything).Return(wantURL, nil)

	h := interfaces.NewHandler(uploader, deleter)
	r := buildRouter(h)

	req, _ := multipartRequest(productID, "image", "photo.jpg", []byte("data"), "image/jpeg")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)

	var resp httputil.Response
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))

	data, _ := json.Marshal(resp.Data)
	var body map[string]string
	require.NoError(t, json.Unmarshal(data, &body))
	assert.Equal(t, wantURL, body["image_url"])
}

func TestUploadImage_MissingField_Returns400(t *testing.T) {
	h := interfaces.NewHandler(&mockUploader{}, &mockDeleter{})
	r := buildRouter(h)

	req := httptest.NewRequest(http.MethodPost, "/admin/products/"+uuid.New().String()+"/image", nil)
	req.Header.Set("Content-Type", "multipart/form-data; boundary=xxxx")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestUploadImage_InvalidUUID_Returns400(t *testing.T) {
	h := interfaces.NewHandler(&mockUploader{}, &mockDeleter{})
	r := buildRouter(h)

	req := httptest.NewRequest(http.MethodPost, "/admin/products/not-a-uuid/image", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestUploadImage_UseCaseNotFound_Returns404(t *testing.T) {
	uploader := &mockUploader{}
	productID := uuid.New()
	uploader.On("Execute", mock.Anything, productID, mock.Anything, mock.Anything, mock.Anything).Return("", sherrors.ErrNotFound)

	h := interfaces.NewHandler(uploader, &mockDeleter{})
	r := buildRouter(h)

	req, _ := multipartRequest(productID, "image", "photo.jpg", []byte("data"), "image/jpeg")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusNotFound, rec.Code)
}

func TestDeleteImage_Success(t *testing.T) {
	deleter := &mockDeleter{}
	productID := uuid.New()
	deleter.On("Execute", mock.Anything, productID).Return(nil)

	h := interfaces.NewHandler(&mockUploader{}, deleter)
	r := buildRouter(h)

	req := httptest.NewRequest(http.MethodDelete, "/admin/products/"+productID.String()+"/image", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusNoContent, rec.Code)
}

func TestDeleteImage_NotFound_Returns404(t *testing.T) {
	deleter := &mockDeleter{}
	productID := uuid.New()
	deleter.On("Execute", mock.Anything, productID).Return(sherrors.ErrNotFound)

	h := interfaces.NewHandler(&mockUploader{}, deleter)
	r := buildRouter(h)

	req := httptest.NewRequest(http.MethodDelete, "/admin/products/"+productID.String()+"/image", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusNotFound, rec.Code)
}
