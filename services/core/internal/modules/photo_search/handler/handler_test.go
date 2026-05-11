package handler_test

import (
	"bytes"
	"context"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/textproto"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	catalogdomain "github.com/foodsea/core/internal/modules/catalog/domain"
	"github.com/foodsea/core/internal/modules/photo_search/domain"
	"github.com/foodsea/core/internal/modules/photo_search/handler"
	"github.com/foodsea/core/internal/platform/httputil"
)

func init() { gin.SetMode(gin.TestMode) }

type mockSearchByPhoto struct{ mock.Mock }

func (m *mockSearchByPhoto) Execute(ctx context.Context, req domain.SearchByPhotoRequest) (domain.SearchResult, error) {
	args := m.Called(ctx, req)
	result, _ := args.Get(0).(domain.SearchResult)
	return result, args.Error(1)
}

func buildRouter(h *handler.Handler) *gin.Engine {
	r := gin.New()
	r.POST("/products/photo-search", h.SearchByPhoto)
	return r
}

func newMultipartRequest(t *testing.T, mime string, ocrText string, topK string) *http.Request {
	t.Helper()

	var body bytes.Buffer
	w := multipart.NewWriter(&body)
	partHeader := make(textproto.MIMEHeader)
	partHeader.Set("Content-Disposition", `form-data; name="image"; filename="photo.jpg"`)
	partHeader.Set("Content-Type", mime)
	part, err := w.CreatePart(partHeader)
	require.NoError(t, err)
	_, err = part.Write([]byte("img-bytes"))
	require.NoError(t, err)
	require.NoError(t, w.WriteField("ocr_text", ocrText))
	if topK != "" {
		require.NoError(t, w.WriteField("top_k", topK))
	}
	require.NoError(t, w.Close())

	req := httptest.NewRequest(http.MethodPost, "/products/photo-search", &body)
	req.Header.Set("Content-Type", w.FormDataContentType())
	return req
}

func TestSearchByPhoto_Success(t *testing.T) {
	exec := &mockSearchByPhoto{}
	h := handler.NewHandler(exec, 8*1024*1024)
	r := buildRouter(h)

	id := uuid.New()
	exec.On("Execute", mock.Anything, mock.MatchedBy(func(req domain.SearchByPhotoRequest) bool {
		return req.TopK == 7 && req.ImageMimeType == "image/png" && req.OCRText == "молоко"
	})).Return(domain.SearchResult{
		MatchedName:  "молоко",
		MatchedBrand: "бренд",
		Candidates: []domain.ProductCandidate{
			{
				Product: &catalogdomain.ProductDetail{
					Product:  catalogdomain.Product{ID: id, Name: "Молоко", InStock: true},
					Category: catalogdomain.Category{ID: uuid.New(), Name: "Молочное"},
				},
				Score:  0.91,
				Source: "image_ocr",
			},
		},
	}, nil)

	req := newMultipartRequest(t, "image/png", "молоко", "7")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	var resp httputil.Response
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	dataMap, ok := resp.Data.(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "молоко", dataMap["matched_name"])
}

func TestSearchByPhoto_MimeRejection(t *testing.T) {
	h := handler.NewHandler(&mockSearchByPhoto{}, 8*1024*1024)
	r := buildRouter(h)

	req := newMultipartRequest(t, "image/webp", "молоко", "")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusUnsupportedMediaType, rec.Code)
}

func TestSearchByPhoto_InvalidTopK(t *testing.T) {
	h := handler.NewHandler(&mockSearchByPhoto{}, 8*1024*1024)
	r := buildRouter(h)

	req := newMultipartRequest(t, "image/jpeg", "молоко", "abc")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusUnprocessableEntity, rec.Code)
}
