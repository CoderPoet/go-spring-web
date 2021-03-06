/*
 * Copyright 2012-2019 the original author or authors.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *      https://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package SpringWeb

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"reflect"
	"time"

	"github.com/go-spring/go-spring-parent/spring-logger"
	"github.com/go-spring/go-spring-parent/spring-utils"
	"github.com/swaggo/http-swagger"
)

// HandlerFunc 标准 Web 处理函数
type HandlerFunc func(WebContext)

// Handler Web 处理接口
type Handler interface {
	// Invoke 响应函数
	Invoke(WebContext)

	// FileLine 获取用户函数的文件名、行号以及函数名称
	FileLine() (file string, line int, fnName string)
}

// ContainerConfig Web 容器配置
type ContainerConfig struct {
	IP        string // 监听 IP
	Port      int    // 监听端口
	EnableSSL bool   // 使用 SSL
	KeyFile   string // SSL 证书
	CertFile  string // SSL 秘钥

	ReadTimeout  time.Duration
	WriteTimeout time.Duration
}

// WebContainer Web 容器
type WebContainer interface {
	// WebMapping 路由表
	WebMapping

	// Config 获取 Web 容器配置
	Config() ContainerConfig

	// GetFilters 返回过滤器列表
	GetFilters() []Filter

	// ResetFilters 重新设置过滤器列表
	ResetFilters(filters []Filter)

	// AddFilter 添加过滤器
	AddFilter(filter ...Filter)

	// GetLoggerFilter 获取 Logger Filter
	GetLoggerFilter() Filter

	// SetLoggerFilter 设置 Logger Filter
	SetLoggerFilter(filter Filter)

	// GetRecoveryFilter 获取 Recovery Filter
	GetRecoveryFilter() Filter

	// SetRecoveryFilter 设置 Recovery Filter
	SetRecoveryFilter(filter Filter)

	// AddRouter 添加新的路由信息
	AddRouter(router *Router)

	// EnableSwagger 是否启用 Swagger 功能
	EnableSwagger() bool

	// SetEnableSwagger 设置是否启用 Swagger 功能
	SetEnableSwagger(enable bool)

	// Start 启动 Web 容器，非阻塞
	Start()

	// Stop 停止 Web 容器，阻塞
	Stop(ctx context.Context)
}

// BaseWebContainer WebContainer 的通用部分
type BaseWebContainer struct {
	WebMapping

	config    ContainerConfig
	filters   []Filter
	enableSwg bool // 是否启用 Swagger 功能

	loggerFilter   Filter // 日志过滤器
	recoveryFilter Filter // 恢复过滤器
}

// NewBaseWebContainer BaseWebContainer 的构造函数
func NewBaseWebContainer(config ContainerConfig) *BaseWebContainer {
	return &BaseWebContainer{
		WebMapping:     NewDefaultWebMapping(),
		config:         config,
		enableSwg:      true,
		loggerFilter:   defaultLoggerFilter,
		recoveryFilter: defaultRecoveryFilter,
	}
}

// Address 返回监听地址
func (c *BaseWebContainer) Address() string {
	return fmt.Sprintf("%s:%d", c.config.IP, c.config.Port)
}

// Config 获取 Web 容器配置
func (c *BaseWebContainer) Config() ContainerConfig {
	return c.config
}

// GetFilters 返回过滤器列表
func (c *BaseWebContainer) GetFilters() []Filter {
	return c.filters
}

// ResetFilters 重新设置过滤器列表
func (c *BaseWebContainer) ResetFilters(filters []Filter) {
	c.filters = filters
}

// AddFilter 添加过滤器
func (c *BaseWebContainer) AddFilter(filter ...Filter) {
	c.filters = append(c.filters, filter...)
}

// GetLoggerFilter 获取 Logger Filter
func (c *BaseWebContainer) GetLoggerFilter() Filter {
	return c.loggerFilter
}

// SetLoggerFilter 设置 Logger Filter
func (c *BaseWebContainer) SetLoggerFilter(filter Filter) {
	c.loggerFilter = filter
}

// GetRecoveryFilter 获取 Recovery Filter
func (c *BaseWebContainer) GetRecoveryFilter() Filter {
	return c.recoveryFilter
}

// 设置 Recovery Filter
func (c *BaseWebContainer) SetRecoveryFilter(filter Filter) {
	c.recoveryFilter = filter
}

// AddRouter 添加新的路由信息
func (c *BaseWebContainer) AddRouter(router *Router) {
	for _, mapper := range router.mapping.Mappers() {
		c.AddMapper(mapper)
	}
}

// EnableSwagger 是否启用 Swagger 功能
func (c *BaseWebContainer) EnableSwagger() bool {
	return c.enableSwg
}

// SetEnableSwagger 设置是否启用 Swagger 功能
func (c *BaseWebContainer) SetEnableSwagger(enable bool) {
	c.enableSwg = enable
}

// PreStart 执行 Start 之前的准备工作
func (c *BaseWebContainer) PreStart() {

	if c.enableSwg {

		// 注册 path 的 Operation
		for _, mapper := range c.Mappers() {
			if op := mapper.swagger; op != nil {
				if err := op.parseBind(); err != nil {
					panic(err)
				}
				doc.AddPath(mapper.Path(), mapper.Method(), op)
			}
		}

		// 注册 swagger-ui 和 doc.json 接口
		c.HandleGet("/swagger/*", HTTP(httpSwagger.Handler(
			httpSwagger.URL("/swagger/doc.json"),
		)))

		// 注册 redoc 接口
		c.GetMapping("/redoc", ReDoc)
	}

}

// PrintMapper 打印路由注册信息
func (c *BaseWebContainer) PrintMapper(m *Mapper) {
	file, line, fnName := m.handler.FileLine()
	SpringLogger.Infof("%v :%d %s -> %s:%d %s", GetMethod(m.method), c.config.Port, m.path, file, line, fnName)
}

/////////////////// Invoke Handler //////////////////////

// InvokeHandler 执行 Web 处理函数
func InvokeHandler(ctx WebContext, fn Handler, filters []Filter) {
	if len(filters) > 0 {
		filters = append(filters, HandlerFilter(fn))
		chain := NewDefaultFilterChain(filters)
		chain.Next(ctx)
	} else {
		fn.Invoke(ctx)
	}
}

/////////////////// Web Handlers //////////////////////

// fnHandler 封装 Web 处理函数
type fnHandler HandlerFunc

func (f fnHandler) Invoke(ctx WebContext) {
	f(ctx)
}

func (f fnHandler) FileLine() (file string, line int, fnName string) {
	return SpringUtils.FileLine(f)
}

// FUNC 标准 Web 处理函数的辅助函数
func FUNC(fn HandlerFunc) Handler {
	return fnHandler(fn)
}

// methodHandler 类型方法处理函数
type methodHandler struct {
	receiver   interface{}
	method     reflect.Value
	methodName string
}

func (m *methodHandler) Invoke(ctx WebContext) {
	m.method.Call([]reflect.Value{reflect.ValueOf(ctx)})
}

func (m *methodHandler) FileLine() (file string, line int, fnName string) {
	method, _ := reflect.TypeOf(m.receiver).MethodByName(m.methodName)
	return SpringUtils.FileLine(method.Func.Interface())
}

// METHOD 和标准 Web 处理函数兼容的对象方法的辅助函数
func METHOD(receiver interface{}, methodName string) Handler {
	method := reflect.ValueOf(receiver).MethodByName(methodName)
	if method.IsZero() {
		panic(errors.New("can't find method " + methodName))
	}
	return &methodHandler{
		receiver:   receiver,
		method:     method,
		methodName: methodName,
	}
}

// httpHandler 标准 Http 处理函数
type httpHandler http.HandlerFunc

func (h httpHandler) Invoke(ctx WebContext) {
	h(ctx.ResponseWriter(), ctx.Request())
}

func (h httpHandler) FileLine() (file string, line int, fnName string) {
	return SpringUtils.FileLine(h)
}

// HTTP 标准 Http 处理函数的辅助函数
func HTTP(fn http.HandlerFunc) Handler {
	return httpHandler(fn)
}

/////////////////// Web Filters //////////////////////

var defaultRecoveryFilter = &recoveryFilter{}

// recoveryFilter 恢复过滤器
type recoveryFilter struct{}

func (f *recoveryFilter) Invoke(ctx WebContext, chain FilterChain) {

	defer func() {
		if err := recover(); err != nil {
			ctx.LogError("[PANIC RECOVER] ", err)
			ctx.Status(http.StatusInternalServerError)
		}
	}()

	chain.Next(ctx)
}

var defaultLoggerFilter = &loggerFilter{}

// loggerFilter 日志过滤器
type loggerFilter struct{}

func (f *loggerFilter) Invoke(ctx WebContext, chain FilterChain) {
	start := time.Now()
	chain.Next(ctx)
	ctx.LogInfo("cost: ", time.Since(start))
}
