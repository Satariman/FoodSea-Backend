package httputil

import (
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/foodsea/ordering/internal/shared/domain"
)

// ParsePagination extracts page and page_size query params and returns a Pagination value object.
func ParsePagination(c *gin.Context) domain.Pagination {
	page, _ := strconv.Atoi(c.Query("page"))
	pageSize, _ := strconv.Atoi(c.Query("page_size"))
	return domain.NewPagination(page, pageSize)
}
