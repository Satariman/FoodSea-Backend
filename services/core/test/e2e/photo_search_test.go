//go:build e2e

package e2e

import (
	"bytes"
	"context"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	pbml "github.com/foodsea/proto/ml"
	"google.golang.org/grpc"
)

func TestPhotoSearchE2E(t *testing.T) {
	access := registerUser(t, "photo-search@foodsea.test", "SuperSecret1!")

	previousSearchFn := testMLClient.searchByPhotoFn
	testMLClient.searchByPhotoFn = func(_ context.Context, req *pbml.SearchByPhotoRequest, _ ...grpc.CallOption) (*pbml.SearchByPhotoResponse, error) {
		require.Equal(t, "image/png", req.GetImageMimeType())
		require.Equal(t, "молоко вкусвилл", req.GetOcrText())
		require.EqualValues(t, 1, req.GetTopK())

		return &pbml.SearchByPhotoResponse{
			MatchedName:  "Молоко 2.5%",
			MatchedBrand: "ВкусВилл",
			Candidates: []*pbml.PhotoSearchCandidate{
				{ProductId: seededProductID, Score: 0.97},
			},
		}, nil
	}
	t.Cleanup(func() {
		testMLClient.searchByPhotoFn = previousSearchFn
	})

	fields := map[string]string{
		"ocr_text": "молоко вкусвилл",
		"top_k":    "1",
	}
	resp, err := postMultipartAuth(
		testBaseURL+"/api/v1/products/photo-search",
		access,
		fields,
		"image",
		"photo.png",
		"image/png",
		[]byte{137, 80, 78, 71, 13, 10, 26, 10},
	)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode)

	var body struct {
		Data struct {
			MatchedName  string `json:"matched_name"`
			MatchedBrand string `json:"matched_brand"`
			Candidates   []struct {
				Product struct {
					ID string `json:"id"`
				} `json:"product"`
				Score float64 `json:"score"`
			} `json:"candidates"`
		} `json:"data"`
	}
	require.NoError(t, decodeJSON(resp, &body))
	assert.Equal(t, "Молоко 2.5%", body.Data.MatchedName)
	assert.Equal(t, "ВкусВилл", body.Data.MatchedBrand)
	require.Len(t, body.Data.Candidates, 1)
	assert.Equal(t, seededProductID, body.Data.Candidates[0].Product.ID)
	assert.InDelta(t, 0.97, body.Data.Candidates[0].Score, 1e-9)
}

func postMultipartAuth(
	url string,
	token string,
	fields map[string]string,
	fileField string,
	fileName string,
	fileContentType string,
	fileContents []byte,
) (*http.Response, error) {
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)

	for key, value := range fields {
		if err := writer.WriteField(key, value); err != nil {
			return nil, err
		}
	}

	header := make(textproto.MIMEHeader)
	header.Set("Content-Disposition", `form-data; name="`+fileField+`"; filename="`+fileName+`"`)
	header.Set("Content-Type", fileContentType)
	part, err := writer.CreatePart(header)
	if err != nil {
		return nil, err
	}
	if _, err = part.Write(fileContents); err != nil {
		return nil, err
	}

	if err = writer.Close(); err != nil {
		return nil, err
	}

	req, err := http.NewRequest(http.MethodPost, url, &body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.Header.Set("Authorization", "Bearer "+token)
	return httpClient.Do(req)
}
