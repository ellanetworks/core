package datarepository

import (
	"github.com/gin-gonic/gin"
	logger_util "github.com/omec-project/util/logger"
	"github.com/yeastengine/ella/internal/udr/logger"
)

// Route is the information for every URI.
type Route struct {
	// Name is the name of this Route.
	Name string
	// Method is the string for the HTTP method. e.g., GET, POST etc.
	Method string
	// Pattern is the pattern of the URI.
	Pattern string
	// HandlerFunc is the handler function of this route.
	HandlerFunc gin.HandlerFunc
}

// Routes is the list of the generated Route.
type Routes []Route

// NewRouter returns a new router.
func NewRouter() *gin.Engine {
	router := logger_util.NewGinWithZap(logger.GinLog)
	return router
}
