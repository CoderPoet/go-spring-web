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

package SpringEcho_test

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"testing"
	"time"

	"github.com/go-spring/go-spring-web/spring-echo"
	"github.com/go-spring/go-spring-web/spring-web"
	"github.com/go-spring/go-spring-parent/spring-utils"
)

type NumberFilter struct {
	n int
}

func NewNumberFilter(n int) *NumberFilter {
	return &NumberFilter{
		n: n,
	}
}

func (f *NumberFilter) Invoke(ctx SpringWeb.WebContext, chain *SpringWeb.FilterChain) {
	defer fmt.Println("::after", f.n)
	fmt.Println("::before", f.n)
	chain.Next(ctx)
}

type Service struct {
	store map[string]string
}

func NewService() *Service {
	return &Service{
		store: make(map[string]string),
	}
}

func (s *Service) Get(ctx SpringWeb.WebContext) {

	key := ctx.QueryParam("key")
	ctx.LogInfo("/get", "key=", key)

	val := s.store[key]
	ctx.LogInfo("/get", "val=", val)

	ctx.String(http.StatusOK, val)
}

func (s *Service) Set(ctx SpringWeb.WebContext) {

	var param struct {
		A string `form:"a" json:"a"`
	}

	ctx.Bind(&param)

	ctx.LogInfo("/set", "param="+SpringUtils.ToJson(param))

	s.store["a"] = param.A
}

func (s *Service) Panic(ctx SpringWeb.WebContext) {
	panic("this is a panic")
}

func TestContainer(t *testing.T) {
	c := SpringEcho.NewContainer()

	s := NewService()

	f2 := NewNumberFilter(2)
	f5 := NewNumberFilter(5)
	f7 := NewNumberFilter(7)

	c.GET("/get", s.Get, f2, f5)

	if false { // 流式风格
		c.Route("", f2, f7).
			POST("/set", s.Set).
			GET("/panic", s.Panic)
	}

	// 障眼法，显得更整齐
	r := c.Route("", f2, f7)
	{
		r.POST("/set", s.Set)
		r.GET("/panic", s.Panic)
	}

	go c.Start(":8080")

	time.Sleep(time.Millisecond * 100)
	fmt.Println()

	resp, _ := http.Get("http://127.0.0.1:8080/get?key=a")
	body, _ := ioutil.ReadAll(resp.Body)
	fmt.Println("code:", resp.StatusCode, "||", "resp:", string(body))
	fmt.Println()

	http.PostForm("http://127.0.0.1:8080/set", url.Values{
		"a": []string{"1"},
	})

	fmt.Println()

	resp, _ = http.Get("http://127.0.0.1:8080/get?key=a")
	body, _ = ioutil.ReadAll(resp.Body)
	fmt.Println("code:", resp.StatusCode, "||", "resp:", string(body))
	fmt.Println()

	resp, _ = http.Get("http://127.0.0.1:8080/panic")
	body, _ = ioutil.ReadAll(resp.Body)
	fmt.Println("code:", resp.StatusCode, "||", "resp:", string(body))
}
