// This file was generated by counterfeiter
package efsdriverfakes

import (
	"sync"

	"code.cloudfoundry.org/efsdriver"
)

type FakeMounter struct {
	MountStub        func(source string, target string, fstype string, flags uintptr, data string) (err error)
	mountMutex       sync.RWMutex
	mountArgsForCall []struct {
		source string
		target string
		fstype string
		flags  uintptr
		data   string
	}
	mountReturns struct {
		result1 error
	}
	UnmountStub        func(target string, flags int) (err error)
	unmountMutex       sync.RWMutex
	unmountArgsForCall []struct {
		target string
		flags  int
	}
	unmountReturns struct {
		result1 error
	}
}

func (fake *FakeMounter) Mount(source string, target string, fstype string, flags uintptr, data string) (err error) {
	fake.mountMutex.Lock()
	fake.mountArgsForCall = append(fake.mountArgsForCall, struct {
		source string
		target string
		fstype string
		flags  uintptr
		data   string
	}{source, target, fstype, flags, data})
	fake.mountMutex.Unlock()
	if fake.MountStub != nil {
		return fake.MountStub(source, target, fstype, flags, data)
	} else {
		return fake.mountReturns.result1
	}
}

func (fake *FakeMounter) MountCallCount() int {
	fake.mountMutex.RLock()
	defer fake.mountMutex.RUnlock()
	return len(fake.mountArgsForCall)
}

func (fake *FakeMounter) MountArgsForCall(i int) (string, string, string, uintptr, string) {
	fake.mountMutex.RLock()
	defer fake.mountMutex.RUnlock()
	return fake.mountArgsForCall[i].source, fake.mountArgsForCall[i].target, fake.mountArgsForCall[i].fstype, fake.mountArgsForCall[i].flags, fake.mountArgsForCall[i].data
}

func (fake *FakeMounter) MountReturns(result1 error) {
	fake.MountStub = nil
	fake.mountReturns = struct {
		result1 error
	}{result1}
}

func (fake *FakeMounter) Unmount(target string, flags int) (err error) {
	fake.unmountMutex.Lock()
	fake.unmountArgsForCall = append(fake.unmountArgsForCall, struct {
		target string
		flags  int
	}{target, flags})
	fake.unmountMutex.Unlock()
	if fake.UnmountStub != nil {
		return fake.UnmountStub(target, flags)
	} else {
		return fake.unmountReturns.result1
	}
}

func (fake *FakeMounter) UnmountCallCount() int {
	fake.unmountMutex.RLock()
	defer fake.unmountMutex.RUnlock()
	return len(fake.unmountArgsForCall)
}

func (fake *FakeMounter) UnmountArgsForCall(i int) (string, int) {
	fake.unmountMutex.RLock()
	defer fake.unmountMutex.RUnlock()
	return fake.unmountArgsForCall[i].target, fake.unmountArgsForCall[i].flags
}

func (fake *FakeMounter) UnmountReturns(result1 error) {
	fake.UnmountStub = nil
	fake.unmountReturns = struct {
		result1 error
	}{result1}
}

var _ efsdriver.Mounter = new(FakeMounter)
