package images

import (
	"log/slog"

	"github.com/gin-gonic/gin"

	"github.com/foodsea/core/ent"
	"github.com/foodsea/core/internal/modules/images/infrastructure"
	"github.com/foodsea/core/internal/modules/images/interfaces"
	"github.com/foodsea/core/internal/modules/images/usecase"
	s3platform "github.com/foodsea/core/internal/platform/s3"
)

// Deps holds the external dependencies for the images module.
type Deps struct {
	EntClient *ent.Client
	S3Client  *s3platform.Client
	Logger    *slog.Logger
}

// Module is the DI container for the images module.
type Module struct {
	handler *interfaces.Handler
}

// NewModule wires all image module dependencies.
func NewModule(deps Deps) *Module {
	repo := infrastructure.NewProductImageRepo(deps.EntClient)

	uploadUC := usecase.NewUploadImage(deps.S3Client, repo, deps.Logger)
	deleteUC := usecase.NewDeleteImage(deps.S3Client, repo, deps.Logger)

	return &Module{
		handler: interfaces.NewHandler(uploadUC, deleteUC),
	}
}

// RegisterRoutes mounts admin image routes onto the given router group.
func (m *Module) RegisterRoutes(admin *gin.RouterGroup) {
	admin.POST("/products/:id/image", m.handler.UploadImage)
	admin.DELETE("/products/:id/image", m.handler.DeleteImage)
}
