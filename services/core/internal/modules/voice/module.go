package voice

import (
	"time"

	"github.com/gin-gonic/gin"
	googlegrpc "google.golang.org/grpc"

	voicegrpc "github.com/foodsea/core/internal/modules/voice/grpc"
	"github.com/foodsea/core/internal/modules/voice/handler"
	"github.com/foodsea/core/internal/modules/voice/usecase"
)

// Deps holds external dependencies required to build the voice module.
type Deps struct {
	MLVoiceConn    *googlegrpc.ClientConn
	RequestTimeout time.Duration
}

// Module is the DI container for the voice module.
type Module struct {
	ParseVoice *usecase.ParseVoice

	httpHandler *handler.Handler
}

// NewModule wires all voice dependencies and returns a ready-to-use Module.
func NewModule(deps Deps) *Module {
	client := voicegrpc.NewMLVoiceClient(deps.MLVoiceConn, deps.RequestTimeout)
	parseVoiceUC := usecase.NewParseVoice(client)
	h := handler.NewHandler(parseVoiceUC)

	return &Module{
		ParseVoice:  parseVoiceUC,
		httpHandler: h,
	}
}

// RegisterRoutes mounts all voice routes onto the protected router group.
func (m *Module) RegisterRoutes(protected *gin.RouterGroup) {
	protected.POST("/voice/parse", m.httpHandler.ParseVoice)
}
