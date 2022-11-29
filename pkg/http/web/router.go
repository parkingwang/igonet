package web

import (
	"fmt"
	"net/http"
	"os"
	"reflect"
	"runtime"
	"text/tabwriter"

	"github.com/gin-gonic/gin"
)

type Router interface {
	// rpc模式路由方法
	// handler 支持rpc方法和gin.HandleFunc(为了支持gin中间件)
	Get(path string, handler ...any)
	Post(path string, handler ...any)
	Put(path string, handler ...any)
	Patch(path string, handler ...any)
	Delete(path string, handler ...any)
	Handle(method, path string, handler ...any)
	// 同gin
	Use(handler ...gin.HandlerFunc) Router
	Group(path string, handler ...gin.HandlerFunc) Router
}

type route struct {
	opt      *option
	basepath string
	r        gin.IRoutes
	isDir    bool
}

func (s *route) Get(path string, handler ...any) {
	s.Handle(http.MethodGet, path, handler...)
}

func (s *route) Post(path string, handler ...any) {
	s.Handle(http.MethodPost, path, handler...)
}

func (s *route) Put(path string, handler ...any) {
	s.Handle(http.MethodPut, path, handler...)
}

func (s *route) Patch(path string, handler ...any) {
	s.Handle(http.MethodPatch, path, handler...)
}

func (s *route) Delete(path string, handler ...any) {
	s.Handle(http.MethodDelete, path, handler...)
}

func (s *route) Use(handler ...gin.HandlerFunc) Router {
	s2 := *s
	s2.r = s.r.Use(handler...)
	return &s2
}

func (s *route) Group(path string, handler ...gin.HandlerFunc) Router {
	r := s.r.(gin.IRouter).Group(path, handler...)
	s.opt.routes.addGroup(r.BasePath())
	return &route{
		opt:      s.opt,
		r:        r,
		basepath: r.BasePath(),
		isDir:    true,
	}
}

func (s *route) Handle(method, path string, handler ...any) {
	hs := make([]gin.HandlerFunc, len(handler))
	var has bool
	for i, h := range handler {
		ginFunc, ok := h.(func(*gin.Context))
		if ok {
			hs[i] = ginFunc
		} else {
			if has {
				panic("handle only support one rpc handler")
			}
			// 添加到路由信息表 为了自动生成doc
			s.opt.routes.addRoute(s.basepath, path, h, method)
			// 使用handleWarpf 转为gin.HandleFunc
			hs[i] = handleWarpf(s.opt)(h)
			has = true
		}
	}

	s.r.Handle(method, path, hs...)
}

type routeInfo struct {
	isDir    bool
	basePath string
	path     string
	// comment  string
	// handle only
	pcName  string
	method  string
	funType reflect.Value
	// dir only
	children Routes
}

type Routes []routeInfo

func (r *Routes) addRoute(basepath, path string, h any, method string) {
	name := runtime.FuncForPC(reflect.ValueOf(h).Pointer()).Name()
	// ns := strings.Split(name, "/")
	info := routeInfo{
		path:     path,
		basePath: basepath,
		// pcName:   ns[len(ns)-1],
		pcName:  name,
		method:  method,
		funType: reflect.ValueOf(h),
	}
	if basepath != "" {
		for k, v := range *r {
			if v.isDir && v.basePath == basepath {
				(*r)[k].children = append((*r)[k].children, info)
				break
			}
		}
	} else {
		*r = append(*r, info)
	}
}

func (r *Routes) addGroup(path string) {
	*r = append(*r, routeInfo{
		isDir:    true,
		basePath: path,
		children: make([]routeInfo, 0),
	})
}

func (r *Routes) echo() {
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', tabwriter.DiscardEmptyColumns)
	for _, v := range *r {
		if v.isDir {
			if len(v.children) == 0 {
				continue
			}
			fmt.Fprintf(w, "[router]├── %s\t\t\n", v.basePath)
			for _, h := range v.children {
				fmt.Fprintf(w, "[router]│   └── %s\t%s\t%s\n", h.path, h.method, h.pcName)
			}
		} else {
			fmt.Fprintf(w, "[router]├── %s\t%s\t%s\n", v.path, v.method, v.pcName)
		}
	}

	w.Flush()
}