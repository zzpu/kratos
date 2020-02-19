// Code generated by protoc-gen-bm v0.1, DO NOT EDIT.
// source: api.proto

package api

import (
	"context"

	bm "github.com/zzpu/kratos/pkg/net/http/blademaster"
	"github.com/zzpu/kratos/pkg/net/http/blademaster/binding"
)
import google_protobuf1 "github.com/golang/protobuf/ptypes/empty"

// to suppressed 'imported but not used warning'
var _ *bm.Context
var _ context.Context
var _ binding.StructValidator

var PathUserInfo = "/user.api.User/Info"
var PathUserCard = "/user.api.User/Card"

// UserBMServer is the server API for User service.
type UserBMServer interface {
	Info(ctx context.Context, req *UserReq) (resp *InfoReply, err error)

	Card(ctx context.Context, req *UserReq) (resp *google_protobuf1.Empty, err error)
}

var UserSvc UserBMServer

func userInfo(c *bm.Context) {
	p := new(UserReq)
	if err := c.BindWith(p, binding.Default(c.Request.Method, c.Request.Header.Get("Content-Type"))); err != nil {
		return
	}
	resp, err := UserSvc.Info(c, p)
	c.JSON(resp, err)
}

func userCard(c *bm.Context) {
	p := new(UserReq)
	if err := c.BindWith(p, binding.Default(c.Request.Method, c.Request.Header.Get("Content-Type"))); err != nil {
		return
	}
	resp, err := UserSvc.Card(c, p)
	c.JSON(resp, err)
}

// RegisterUserBMServer Register the blademaster route
func RegisterUserBMServer(e *bm.Engine, server UserBMServer) {
	UserSvc = server
	e.GET("/user.api.User/Info", userInfo)
	e.GET("/user.api.User/Card", userCard)
}
