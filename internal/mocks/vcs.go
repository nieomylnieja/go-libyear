// Code generated by MockGen. DO NOT EDIT.
// Source: github.com/nieomylnieja/go-libyear (interfaces: VCSHandler)
//
// Generated by this command:
//
//	mockgen -destination internal/mocks/vcs.go -package mocks -typed . VCSHandler
//

// Package mocks is a generated GoMock package.
package mocks

import (
	reflect "reflect"

	semver "github.com/Masterminds/semver"
	gomock "go.uber.org/mock/gomock"

	internal "github.com/nieomylnieja/go-libyear/internal"
)

// MockVCSHandler is a mock of VCSHandler interface.
type MockVCSHandler struct {
	ctrl     *gomock.Controller
	recorder *MockVCSHandlerMockRecorder
}

// MockVCSHandlerMockRecorder is the mock recorder for MockVCSHandler.
type MockVCSHandlerMockRecorder struct {
	mock *MockVCSHandler
}

// NewMockVCSHandler creates a new mock instance.
func NewMockVCSHandler(ctrl *gomock.Controller) *MockVCSHandler {
	mock := &MockVCSHandler{ctrl: ctrl}
	mock.recorder = &MockVCSHandlerMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockVCSHandler) EXPECT() *MockVCSHandlerMockRecorder {
	return m.recorder
}

// CanHandle mocks base method.
func (m *MockVCSHandler) CanHandle(arg0 string) (bool, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "CanHandle", arg0)
	ret0, _ := ret[0].(bool)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// CanHandle indicates an expected call of CanHandle.
func (mr *MockVCSHandlerMockRecorder) CanHandle(arg0 any) *MockVCSHandlerCanHandleCall {
	mr.mock.ctrl.T.Helper()
	call := mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "CanHandle", reflect.TypeOf((*MockVCSHandler)(nil).CanHandle), arg0)
	return &MockVCSHandlerCanHandleCall{Call: call}
}

// MockVCSHandlerCanHandleCall wrap *gomock.Call
type MockVCSHandlerCanHandleCall struct {
	*gomock.Call
}

// Return rewrite *gomock.Call.Return
func (c *MockVCSHandlerCanHandleCall) Return(arg0 bool, arg1 error) *MockVCSHandlerCanHandleCall {
	c.Call = c.Call.Return(arg0, arg1)
	return c
}

// Do rewrite *gomock.Call.Do
func (c *MockVCSHandlerCanHandleCall) Do(f func(string) (bool, error)) *MockVCSHandlerCanHandleCall {
	c.Call = c.Call.Do(f)
	return c
}

// DoAndReturn rewrite *gomock.Call.DoAndReturn
func (c *MockVCSHandlerCanHandleCall) DoAndReturn(f func(string) (bool, error)) *MockVCSHandlerCanHandleCall {
	c.Call = c.Call.DoAndReturn(f)
	return c
}

// GetInfo mocks base method.
func (m *MockVCSHandler) GetInfo(arg0 string, arg1 *semver.Version) (*internal.Module, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetInfo", arg0, arg1)
	ret0, _ := ret[0].(*internal.Module)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// GetInfo indicates an expected call of GetInfo.
func (mr *MockVCSHandlerMockRecorder) GetInfo(arg0, arg1 any) *MockVCSHandlerGetInfoCall {
	mr.mock.ctrl.T.Helper()
	call := mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetInfo", reflect.TypeOf((*MockVCSHandler)(nil).GetInfo), arg0, arg1)
	return &MockVCSHandlerGetInfoCall{Call: call}
}

// MockVCSHandlerGetInfoCall wrap *gomock.Call
type MockVCSHandlerGetInfoCall struct {
	*gomock.Call
}

// Return rewrite *gomock.Call.Return
func (c *MockVCSHandlerGetInfoCall) Return(arg0 *internal.Module, arg1 error) *MockVCSHandlerGetInfoCall {
	c.Call = c.Call.Return(arg0, arg1)
	return c
}

// Do rewrite *gomock.Call.Do
func (c *MockVCSHandlerGetInfoCall) Do(f func(string, *semver.Version) (*internal.Module, error)) *MockVCSHandlerGetInfoCall {
	c.Call = c.Call.Do(f)
	return c
}

// DoAndReturn rewrite *gomock.Call.DoAndReturn
func (c *MockVCSHandlerGetInfoCall) DoAndReturn(f func(string, *semver.Version) (*internal.Module, error)) *MockVCSHandlerGetInfoCall {
	c.Call = c.Call.DoAndReturn(f)
	return c
}

// GetLatestInfo mocks base method.
func (m *MockVCSHandler) GetLatestInfo(arg0 string) (*internal.Module, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetLatestInfo", arg0)
	ret0, _ := ret[0].(*internal.Module)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// GetLatestInfo indicates an expected call of GetLatestInfo.
func (mr *MockVCSHandlerMockRecorder) GetLatestInfo(arg0 any) *MockVCSHandlerGetLatestInfoCall {
	mr.mock.ctrl.T.Helper()
	call := mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetLatestInfo", reflect.TypeOf((*MockVCSHandler)(nil).GetLatestInfo), arg0)
	return &MockVCSHandlerGetLatestInfoCall{Call: call}
}

// MockVCSHandlerGetLatestInfoCall wrap *gomock.Call
type MockVCSHandlerGetLatestInfoCall struct {
	*gomock.Call
}

// Return rewrite *gomock.Call.Return
func (c *MockVCSHandlerGetLatestInfoCall) Return(arg0 *internal.Module, arg1 error) *MockVCSHandlerGetLatestInfoCall {
	c.Call = c.Call.Return(arg0, arg1)
	return c
}

// Do rewrite *gomock.Call.Do
func (c *MockVCSHandlerGetLatestInfoCall) Do(f func(string) (*internal.Module, error)) *MockVCSHandlerGetLatestInfoCall {
	c.Call = c.Call.Do(f)
	return c
}

// DoAndReturn rewrite *gomock.Call.DoAndReturn
func (c *MockVCSHandlerGetLatestInfoCall) DoAndReturn(f func(string) (*internal.Module, error)) *MockVCSHandlerGetLatestInfoCall {
	c.Call = c.Call.DoAndReturn(f)
	return c
}

// GetModFile mocks base method.
func (m *MockVCSHandler) GetModFile(arg0 string, arg1 *semver.Version) ([]byte, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetModFile", arg0, arg1)
	ret0, _ := ret[0].([]byte)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// GetModFile indicates an expected call of GetModFile.
func (mr *MockVCSHandlerMockRecorder) GetModFile(arg0, arg1 any) *MockVCSHandlerGetModFileCall {
	mr.mock.ctrl.T.Helper()
	call := mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetModFile", reflect.TypeOf((*MockVCSHandler)(nil).GetModFile), arg0, arg1)
	return &MockVCSHandlerGetModFileCall{Call: call}
}

// MockVCSHandlerGetModFileCall wrap *gomock.Call
type MockVCSHandlerGetModFileCall struct {
	*gomock.Call
}

// Return rewrite *gomock.Call.Return
func (c *MockVCSHandlerGetModFileCall) Return(arg0 []byte, arg1 error) *MockVCSHandlerGetModFileCall {
	c.Call = c.Call.Return(arg0, arg1)
	return c
}

// Do rewrite *gomock.Call.Do
func (c *MockVCSHandlerGetModFileCall) Do(f func(string, *semver.Version) ([]byte, error)) *MockVCSHandlerGetModFileCall {
	c.Call = c.Call.Do(f)
	return c
}

// DoAndReturn rewrite *gomock.Call.DoAndReturn
func (c *MockVCSHandlerGetModFileCall) DoAndReturn(f func(string, *semver.Version) ([]byte, error)) *MockVCSHandlerGetModFileCall {
	c.Call = c.Call.DoAndReturn(f)
	return c
}

// GetVersions mocks base method.
func (m *MockVCSHandler) GetVersions(arg0 string) ([]*semver.Version, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetVersions", arg0)
	ret0, _ := ret[0].([]*semver.Version)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// GetVersions indicates an expected call of GetVersions.
func (mr *MockVCSHandlerMockRecorder) GetVersions(arg0 any) *MockVCSHandlerGetVersionsCall {
	mr.mock.ctrl.T.Helper()
	call := mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetVersions", reflect.TypeOf((*MockVCSHandler)(nil).GetVersions), arg0)
	return &MockVCSHandlerGetVersionsCall{Call: call}
}

// MockVCSHandlerGetVersionsCall wrap *gomock.Call
type MockVCSHandlerGetVersionsCall struct {
	*gomock.Call
}

// Return rewrite *gomock.Call.Return
func (c *MockVCSHandlerGetVersionsCall) Return(arg0 []*semver.Version, arg1 error) *MockVCSHandlerGetVersionsCall {
	c.Call = c.Call.Return(arg0, arg1)
	return c
}

// Do rewrite *gomock.Call.Do
func (c *MockVCSHandlerGetVersionsCall) Do(f func(string) ([]*semver.Version, error)) *MockVCSHandlerGetVersionsCall {
	c.Call = c.Call.Do(f)
	return c
}

// DoAndReturn rewrite *gomock.Call.DoAndReturn
func (c *MockVCSHandlerGetVersionsCall) DoAndReturn(f func(string) ([]*semver.Version, error)) *MockVCSHandlerGetVersionsCall {
	c.Call = c.Call.DoAndReturn(f)
	return c
}

// Name mocks base method.
func (m *MockVCSHandler) Name() string {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Name")
	ret0, _ := ret[0].(string)
	return ret0
}

// Name indicates an expected call of Name.
func (mr *MockVCSHandlerMockRecorder) Name() *MockVCSHandlerNameCall {
	mr.mock.ctrl.T.Helper()
	call := mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Name", reflect.TypeOf((*MockVCSHandler)(nil).Name))
	return &MockVCSHandlerNameCall{Call: call}
}

// MockVCSHandlerNameCall wrap *gomock.Call
type MockVCSHandlerNameCall struct {
	*gomock.Call
}

// Return rewrite *gomock.Call.Return
func (c *MockVCSHandlerNameCall) Return(arg0 string) *MockVCSHandlerNameCall {
	c.Call = c.Call.Return(arg0)
	return c
}

// Do rewrite *gomock.Call.Do
func (c *MockVCSHandlerNameCall) Do(f func() string) *MockVCSHandlerNameCall {
	c.Call = c.Call.Do(f)
	return c
}

// DoAndReturn rewrite *gomock.Call.DoAndReturn
func (c *MockVCSHandlerNameCall) DoAndReturn(f func() string) *MockVCSHandlerNameCall {
	c.Call = c.Call.DoAndReturn(f)
	return c
}