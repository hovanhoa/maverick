package http

import (
	"github.com/gin-gonic/gin"
)

// HandlerFunc defines the type of function that can be used to implement middleware
// and handle incoming HTTP requests
type HandlerFunc func(c *Context)

type Router struct {
	router gin.IRouter
}

type IRouter interface {
	Use(handlers ...HandlerFunc) IRouter
	Handle(method string, path string, handlers ...HandlerFunc) IRouter
	Any(path string, handlers ...HandlerFunc) IRouter
	GET(path string, handlers ...HandlerFunc) IRouter
	POST(path string, handlers ...HandlerFunc) IRouter
	DELETE(path string, handlers ...HandlerFunc) IRouter
	PATCH(path string, handlers ...HandlerFunc) IRouter
	PUT(path string, handlers ...HandlerFunc) IRouter
	OPTIONS(path string, handlers ...HandlerFunc) IRouter
	HEAD(path string, handlers ...HandlerFunc) IRouter
	Group(path string) IRouterGroup
}

type IRouterGroup interface {
	IRouter
	BasePath() string
}

var _ IRouter = (*Router)(nil)
var _ IRouterGroup = (*RouterGroup)(nil)

func (r *Router) Use(handlers ...HandlerFunc) IRouter {
	r.router.Use(getGinHandlers(handlers)...)
	return r
}

func (r *Router) Handle(method string, path string, handlers ...HandlerFunc) IRouter {
	r.router.Handle(method, path, getGinHandlers(handlers)...)
	return r
}

func (r *Router) Any(path string, handlers ...HandlerFunc) IRouter {
	r.router.Any(path, getGinHandlers(handlers)...)
	return r
}

func (r *Router) GET(path string, handlers ...HandlerFunc) IRouter {
	return r.Handle("GET", path, handlers...)
}

func (r *Router) POST(path string, handlers ...HandlerFunc) IRouter {
	return r.Handle("POST", path, handlers...)
}

func (r *Router) DELETE(path string, handlers ...HandlerFunc) IRouter {
	return r.Handle("DELETE", path, handlers...)
}

func (r *Router) PATCH(path string, handlers ...HandlerFunc) IRouter {
	return r.Handle("PATCH", path, handlers...)
}

func (r *Router) PUT(path string, handlers ...HandlerFunc) IRouter {
	return r.Handle("PUT", path, handlers...)
}

func (r *Router) OPTIONS(path string, handlers ...HandlerFunc) IRouter {
	return r.Handle("OPTIONS", path, handlers...)
}

func (r *Router) HEAD(path string, handlers ...HandlerFunc) IRouter {
	return r.Handle("HEAD", path, handlers...)
}

func (r *Router) Group(path string) IRouterGroup {
	group := r.router.Group(path)
	return &RouterGroup{IRouter: &Router{group}, group: group}
}

type RouterGroup struct {
	IRouter
	group *gin.RouterGroup
}

func (rg *RouterGroup) BasePath() string {
	return rg.group.BasePath()
}

func getGinHandlers(handlers []HandlerFunc) (ginHandlers []gin.HandlerFunc) {
	for i := 0; i < len(handlers); i++ {
		ginHandlers = append(ginHandlers, gin.HandlerFunc(handlers[i]))
	}
	return
}
