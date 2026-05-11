package grpc_test

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	grpcstatus "google.golang.org/grpc/status"

	"github.com/foodsea/core/internal/modules/photo_search/domain"
	photogrpc "github.com/foodsea/core/internal/modules/photo_search/grpc"
	sherrors "github.com/foodsea/core/internal/shared/errors"
	pbml "github.com/foodsea/proto/ml"
)

type mockAnalogClient struct{ mock.Mock }

func (m *mockAnalogClient) GetAnalogs(context.Context, *pbml.GetAnalogsRequest, ...grpc.CallOption) (*pbml.GetAnalogsResponse, error) {
	panic("not used")
}

func (m *mockAnalogClient) GetBatchAnalogs(context.Context, *pbml.GetBatchAnalogsRequest, ...grpc.CallOption) (*pbml.GetBatchAnalogsResponse, error) {
	panic("not used")
}

func (m *mockAnalogClient) SearchByPhoto(ctx context.Context, req *pbml.SearchByPhotoRequest, _ ...grpc.CallOption) (*pbml.SearchByPhotoResponse, error) {
	args := m.Called(ctx, req)
	resp, _ := args.Get(0).(*pbml.SearchByPhotoResponse)
	return resp, args.Error(1)
}

func TestMLClient_SearchByPhoto_MapsResponseAndSkipsInvalidIDs(t *testing.T) {
	client := &mockAnalogClient{}
	adapter := photogrpc.NewMLClient(client)

	id := uuid.New()
	client.On("SearchByPhoto", mock.Anything, mock.AnythingOfType("*ml.SearchByPhotoRequest")).Return(&pbml.SearchByPhotoResponse{
		MatchedName:  "молоко",
		MatchedBrand: "бренд",
		Candidates: []*pbml.PhotoSearchCandidate{
			{ProductId: id.String(), Score: 0.8},
			{ProductId: "bad-uuid", Score: 0.99},
		},
	}, nil)

	result, err := adapter.SearchByPhoto(context.Background(), domain.SearchByPhotoRequest{
		Image:         []byte("img"),
		ImageMimeType: "image/jpeg",
		OCRText:       "молоко",
		TopK:          3,
	})
	require.NoError(t, err)
	require.Len(t, result.Candidates, 1)
	assert.Equal(t, id, result.Candidates[0].ProductID)
	assert.Equal(t, 0.8, result.Candidates[0].Score)
	assert.Equal(t, "молоко", result.MatchedName)
}

func TestMLClient_SearchByPhoto_UnavailableMapping(t *testing.T) {
	client := &mockAnalogClient{}
	adapter := photogrpc.NewMLClient(client)

	client.On("SearchByPhoto", mock.Anything, mock.AnythingOfType("*ml.SearchByPhotoRequest")).
		Return((*pbml.SearchByPhotoResponse)(nil), grpcstatus.Error(codes.Unavailable, "down"))

	_, err := adapter.SearchByPhoto(context.Background(), domain.SearchByPhotoRequest{})
	require.Error(t, err)
	assert.True(t, errors.Is(err, sherrors.ErrUnavailable))
}
