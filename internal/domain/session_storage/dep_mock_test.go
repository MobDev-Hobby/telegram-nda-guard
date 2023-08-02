// Code generated by MockGen. DO NOT EDIT.
// Source: dep.go

// Package session_storage is a generated GoMock package.
package session_storage

import (
	reflect "reflect"

	gomock "github.com/golang/mock/gomock"
)

// MockCryproProvider is a mock of CryproProvider interface.
type MockCryproProvider struct {
	ctrl     *gomock.Controller
	recorder *MockCryproProviderMockRecorder
}

// MockCryproProviderMockRecorder is the mock recorder for MockCryproProvider.
type MockCryproProviderMockRecorder struct {
	mock *MockCryproProvider
}

// NewMockCryproProvider creates a new mock instance.
func NewMockCryproProvider(ctrl *gomock.Controller) *MockCryproProvider {
	mock := &MockCryproProvider{ctrl: ctrl}
	mock.recorder = &MockCryproProviderMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockCryproProvider) EXPECT() *MockCryproProviderMockRecorder {
	return m.recorder
}

// BlockSize mocks base method.
func (m *MockCryproProvider) BlockSize() int {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "BlockSize")
	ret0, _ := ret[0].(int)
	return ret0
}

// BlockSize indicates an expected call of BlockSize.
func (mr *MockCryproProviderMockRecorder) BlockSize() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "BlockSize", reflect.TypeOf((*MockCryproProvider)(nil).BlockSize))
}

// Decrypt mocks base method.
func (m *MockCryproProvider) Decrypt(dst, src []byte) {
	m.ctrl.T.Helper()
	m.ctrl.Call(m, "Decrypt", dst, src)
}

// Decrypt indicates an expected call of Decrypt.
func (mr *MockCryproProviderMockRecorder) Decrypt(dst, src interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Decrypt", reflect.TypeOf((*MockCryproProvider)(nil).Decrypt), dst, src)
}

// Encrypt mocks base method.
func (m *MockCryproProvider) Encrypt(dst, src []byte) {
	m.ctrl.T.Helper()
	m.ctrl.Call(m, "Encrypt", dst, src)
}

// Encrypt indicates an expected call of Encrypt.
func (mr *MockCryproProviderMockRecorder) Encrypt(dst, src interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Encrypt", reflect.TypeOf((*MockCryproProvider)(nil).Encrypt), dst, src)
}
