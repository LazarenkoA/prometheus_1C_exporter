// Code generated by MockGen. DO NOT EDIT.
// Source: ./interface.go

// Package mock_model is a generated GoMock package.
package mock_model

import (
	reflect "reflect"

	model "github.com/LazarenkoA/prometheus_1C_exporter/explorers/model"
	gomock "github.com/golang/mock/gomock"
)

// MockIsettings is a mock of Isettings interface.
type MockIsettings struct {
	ctrl     *gomock.Controller
	recorder *MockIsettingsMockRecorder
}

// MockIsettingsMockRecorder is the mock recorder for MockIsettings.
type MockIsettingsMockRecorder struct {
	mock *MockIsettings
}

// NewMockIsettings creates a new mock instance.
func NewMockIsettings(ctrl *gomock.Controller) *MockIsettings {
	mock := &MockIsettings{ctrl: ctrl}
	mock.recorder = &MockIsettingsMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockIsettings) EXPECT() *MockIsettingsMockRecorder {
	return m.recorder
}

// GetExplorers mocks base method.
func (m *MockIsettings) GetExplorers() map[string]map[string]interface{} {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetExplorers")
	ret0, _ := ret[0].(map[string]map[string]interface{})
	return ret0
}

// GetExplorers indicates an expected call of GetExplorers.
func (mr *MockIsettingsMockRecorder) GetExplorers() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetExplorers", reflect.TypeOf((*MockIsettings)(nil).GetExplorers))
}

// GetLogPass mocks base method.
func (m *MockIsettings) GetLogPass(arg0 string) (string, string) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetLogPass", arg0)
	ret0, _ := ret[0].(string)
	ret1, _ := ret[1].(string)
	return ret0, ret1
}

// GetLogPass indicates an expected call of GetLogPass.
func (mr *MockIsettingsMockRecorder) GetLogPass(arg0 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetLogPass", reflect.TypeOf((*MockIsettings)(nil).GetLogPass), arg0)
}

// GetProperty mocks base method.
func (m *MockIsettings) GetProperty(arg0, arg1 string, arg2 interface{}) interface{} {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetProperty", arg0, arg1, arg2)
	ret0, _ := ret[0].(interface{})
	return ret0
}

// GetProperty indicates an expected call of GetProperty.
func (mr *MockIsettingsMockRecorder) GetProperty(arg0, arg1, arg2 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetProperty", reflect.TypeOf((*MockIsettings)(nil).GetProperty), arg0, arg1, arg2)
}

// RAC_Host mocks base method.
func (m *MockIsettings) RAC_Host() string {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "RAC_Host")
	ret0, _ := ret[0].(string)
	return ret0
}

// RAC_Host indicates an expected call of RAC_Host.
func (mr *MockIsettingsMockRecorder) RAC_Host() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "RAC_Host", reflect.TypeOf((*MockIsettings)(nil).RAC_Host))
}

// RAC_Login mocks base method.
func (m *MockIsettings) RAC_Login() string {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "RAC_Login")
	ret0, _ := ret[0].(string)
	return ret0
}

// RAC_Login indicates an expected call of RAC_Login.
func (mr *MockIsettingsMockRecorder) RAC_Login() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "RAC_Login", reflect.TypeOf((*MockIsettings)(nil).RAC_Login))
}

// RAC_Pass mocks base method.
func (m *MockIsettings) RAC_Pass() string {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "RAC_Pass")
	ret0, _ := ret[0].(string)
	return ret0
}

// RAC_Pass indicates an expected call of RAC_Pass.
func (mr *MockIsettingsMockRecorder) RAC_Pass() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "RAC_Pass", reflect.TypeOf((*MockIsettings)(nil).RAC_Pass))
}

// RAC_Path mocks base method.
func (m *MockIsettings) RAC_Path() string {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "RAC_Path")
	ret0, _ := ret[0].(string)
	return ret0
}

// RAC_Path indicates an expected call of RAC_Path.
func (mr *MockIsettingsMockRecorder) RAC_Path() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "RAC_Path", reflect.TypeOf((*MockIsettings)(nil).RAC_Path))
}

// RAC_Port mocks base method.
func (m *MockIsettings) RAC_Port() string {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "RAC_Port")
	ret0, _ := ret[0].(string)
	return ret0
}

// RAC_Port indicates an expected call of RAC_Port.
func (mr *MockIsettingsMockRecorder) RAC_Port() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "RAC_Port", reflect.TypeOf((*MockIsettings)(nil).RAC_Port))
}

// MockIExplorers is a mock of IExplorers interface.
type MockIExplorers struct {
	ctrl     *gomock.Controller
	recorder *MockIExplorersMockRecorder
}

// MockIExplorersMockRecorder is the mock recorder for MockIExplorers.
type MockIExplorersMockRecorder struct {
	mock *MockIExplorers
}

// NewMockIExplorers creates a new mock instance.
func NewMockIExplorers(ctrl *gomock.Controller) *MockIExplorers {
	mock := &MockIExplorers{ctrl: ctrl}
	mock.recorder = &MockIExplorersMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockIExplorers) EXPECT() *MockIExplorersMockRecorder {
	return m.recorder
}

// StartExplore mocks base method.
func (m *MockIExplorers) StartExplore() {
	m.ctrl.T.Helper()
	m.ctrl.Call(m, "StartExplore")
}

// StartExplore indicates an expected call of StartExplore.
func (mr *MockIExplorersMockRecorder) StartExplore() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "StartExplore", reflect.TypeOf((*MockIExplorers)(nil).StartExplore))
}

// MockIexplorer is a mock of Iexplorer interface.
type MockIexplorer struct {
	ctrl     *gomock.Controller
	recorder *MockIexplorerMockRecorder
}

// MockIexplorerMockRecorder is the mock recorder for MockIexplorer.
type MockIexplorerMockRecorder struct {
	mock *MockIexplorer
}

// NewMockIexplorer creates a new mock instance.
func NewMockIexplorer(ctrl *gomock.Controller) *MockIexplorer {
	mock := &MockIexplorer{ctrl: ctrl}
	mock.recorder = &MockIexplorerMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockIexplorer) EXPECT() *MockIexplorerMockRecorder {
	return m.recorder
}

// Continue mocks base method.
func (m *MockIexplorer) Continue(expName string) {
	m.ctrl.T.Helper()
	m.ctrl.Call(m, "Continue", expName)
}

// Continue indicates an expected call of Continue.
func (mr *MockIexplorerMockRecorder) Continue(expName interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Continue", reflect.TypeOf((*MockIexplorer)(nil).Continue), expName)
}

// GetName mocks base method.
func (m *MockIexplorer) GetName() string {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetName")
	ret0, _ := ret[0].(string)
	return ret0
}

// GetName indicates an expected call of GetName.
func (mr *MockIexplorerMockRecorder) GetName() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetName", reflect.TypeOf((*MockIexplorer)(nil).GetName))
}

// Pause mocks base method.
func (m *MockIexplorer) Pause(expName string) {
	m.ctrl.T.Helper()
	m.ctrl.Call(m, "Pause", expName)
}

// Pause indicates an expected call of Pause.
func (mr *MockIexplorerMockRecorder) Pause(expName interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Pause", reflect.TypeOf((*MockIexplorer)(nil).Pause), expName)
}

// Start mocks base method.
func (m *MockIexplorer) Start(arg0 model.IExplorers) {
	m.ctrl.T.Helper()
	m.ctrl.Call(m, "Start", arg0)
}

// Start indicates an expected call of Start.
func (mr *MockIexplorerMockRecorder) Start(arg0 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Start", reflect.TypeOf((*MockIexplorer)(nil).Start), arg0)
}

// StartExplore mocks base method.
func (m *MockIexplorer) StartExplore() {
	m.ctrl.T.Helper()
	m.ctrl.Call(m, "StartExplore")
}

// StartExplore indicates an expected call of StartExplore.
func (mr *MockIexplorerMockRecorder) StartExplore() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "StartExplore", reflect.TypeOf((*MockIexplorer)(nil).StartExplore))
}

// Stop mocks base method.
func (m *MockIexplorer) Stop() {
	m.ctrl.T.Helper()
	m.ctrl.Call(m, "Stop")
}

// Stop indicates an expected call of Stop.
func (mr *MockIexplorerMockRecorder) Stop() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Stop", reflect.TypeOf((*MockIexplorer)(nil).Stop))
}
