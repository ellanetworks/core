package server

import (
	"io/fs"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/yeastengine/ella/internal/logger"
	"github.com/yeastengine/ella/internal/nms/ui"
)

func AddUiService(engine *gin.Engine) {
	staticFilesSystem, err := fs.Sub(ui.FrontendFS, "frontend_files")
	if err != nil {
		logger.NmsLog.Fatal(err)
	}

	engine.Use(func(c *gin.Context) {
		if !isApiUrlPath(c.Request.URL.Path) {
			htmlPath := strings.TrimPrefix(c.Request.URL.Path, "/") + ".html"
			if _, err := staticFilesSystem.Open(htmlPath); err == nil {
				c.Request.URL.Path = htmlPath
			}
			fileServer := http.FileServer(http.FS(staticFilesSystem))
			fileServer.ServeHTTP(c.Writer, c.Request)
			c.Abort()
		}
	})
}

func isApiUrlPath(path string) bool {
	return strings.HasPrefix(path, "/api/v1/")
}
