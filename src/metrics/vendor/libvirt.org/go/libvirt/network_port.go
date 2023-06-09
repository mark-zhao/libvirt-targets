/*
 * This file is part of the libvirt-go-module project
 *
 * Permission is hereby granted, free of charge, to any person obtaining a copy
 * of this software and associated documentation files (the "Software"), to deal
 * in the Software without restriction, including without limitation the rights
 * to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
 * copies of the Software, and to permit persons to whom the Software is
 * furnished to do so, subject to the following conditions:
 *
 * The above copyright notice and this permission notice shall be included in
 * all copies or substantial portions of the Software.
 *
 * THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
 * IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
 * FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
 * AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
 * LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
 * OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN
 * THE SOFTWARE.
 *
 * Copyright (C) 2019 Red Hat, Inc.
 *
 */

package libvirt

/*
#cgo !libvirt_dlopen pkg-config: libvirt
#cgo libvirt_dlopen LDFLAGS: -ldl
#cgo libvirt_dlopen CFLAGS: -DLIBVIRT_DLOPEN
#include <stdlib.h>
#include "libvirt_generated.h"
*/
import "C"

import (
	"unsafe"
)

type NetworkPortCreateFlags uint

const (
	NETWORK_PORT_CREATE_RECLAIM  = NetworkPortCreateFlags(C.VIR_NETWORK_PORT_CREATE_RECLAIM)
	NETWORK_PORT_CREATE_VALIDATE = NetworkPortCreateFlags(C.VIR_NETWORK_PORT_CREATE_VALIDATE)
)

type NetworkPort struct {
	ptr C.virNetworkPortPtr
}

// See also https://libvirt.org/html/libvirt-libvirt-network.html#virNetworkPortFree
func (n *NetworkPort) Free() error {
	var err C.virError
	ret := C.virNetworkPortFreeWrapper(n.ptr, &err)
	if ret == -1 {
		return makeError(&err)
	}
	return nil
}

// See also https://libvirt.org/html/libvirt-libvirt-network.html#virNetworkPortRef
func (c *NetworkPort) Ref() error {
	var err C.virError
	ret := C.virNetworkPortRefWrapper(c.ptr, &err)
	if ret == -1 {
		return makeError(&err)
	}
	return nil
}

// See also https://libvirt.org/html/libvirt-libvirt-network.html#virNetworkPortGetNetwork
//
// Contrary to the native C API behaviour, the Go API will
// acquire a reference on the returned Network, which must
// be released by calling Free()
func (n *NetworkPort) GetNetwork() (*Network, error) {
	var err C.virError
	ptr := C.virNetworkPortGetNetworkWrapper(n.ptr, &err)
	if ptr == nil {
		return nil, makeError(&err)
	}

	ret := C.virNetworkRefWrapper(ptr, &err)
	if ret == -1 {
		return nil, makeError(&err)
	}

	return &Network{ptr: ptr}, nil
}

// See also https://libvirt.org/html/libvirt-libvirt-network.html#virNetworkPortGetUUID
func (n *NetworkPort) GetUUID() ([]byte, error) {
	var cUuid [C.VIR_UUID_BUFLEN](byte)
	cuidPtr := unsafe.Pointer(&cUuid)
	var err C.virError
	result := C.virNetworkPortGetUUIDWrapper(n.ptr, (*C.uchar)(cuidPtr), &err)
	if result != 0 {
		return []byte{}, makeError(&err)
	}
	return C.GoBytes(cuidPtr, C.VIR_UUID_BUFLEN), nil
}

// See also https://libvirt.org/html/libvirt-libvirt-network.html#virNetworkPortGetUUIDString
func (n *NetworkPort) GetUUIDString() (string, error) {
	var cUuid [C.VIR_UUID_STRING_BUFLEN](C.char)
	cuidPtr := unsafe.Pointer(&cUuid)
	var err C.virError
	result := C.virNetworkPortGetUUIDStringWrapper(n.ptr, (*C.char)(cuidPtr), &err)
	if result != 0 {
		return "", makeError(&err)
	}
	return C.GoString((*C.char)(cuidPtr)), nil
}

// See also https://libvirt.org/html/libvirt-libvirt-network.html#virNetworkPortDelete
func (n *NetworkPort) Delete(flags uint32) error {
	var err C.virError
	result := C.virNetworkPortDeleteWrapper(n.ptr, C.uint(flags), &err)
	if result == -1 {
		return makeError(&err)
	}
	return nil
}

// See also https://libvirt.org/html/libvirt-libvirt-network.html#virNetworkPortGetXMLDesc
func (d *NetworkPort) GetXMLDesc(flags uint32) (string, error) {
	var err C.virError
	result := C.virNetworkPortGetXMLDescWrapper(d.ptr, C.uint(flags), &err)
	if result == nil {
		return "", makeError(&err)
	}
	xml := C.GoString(result)
	C.free(unsafe.Pointer(result))
	return xml, nil
}

type NetworkPortParameters struct {
	BandwidthInAverageSet  bool
	BandwidthInAverage     uint
	BandwidthInPeakSet     bool
	BandwidthInPeak        uint
	BandwidthInBurstSet    bool
	BandwidthInBurst       uint
	BandwidthInFloorSet    bool
	BandwidthInFloor       uint
	BandwidthOutAverageSet bool
	BandwidthOutAverage    uint
	BandwidthOutPeakSet    bool
	BandwidthOutPeak       uint
	BandwidthOutBurstSet   bool
	BandwidthOutBurst      uint
}

func getNetworkPortParametersFieldInfo(params *NetworkPortParameters) map[string]typedParamsFieldInfo {
	return map[string]typedParamsFieldInfo{
		C.VIR_NETWORK_PORT_BANDWIDTH_IN_AVERAGE: typedParamsFieldInfo{
			set: &params.BandwidthInAverageSet,
			ui:  &params.BandwidthInAverage,
		},
		C.VIR_NETWORK_PORT_BANDWIDTH_IN_PEAK: typedParamsFieldInfo{
			set: &params.BandwidthInPeakSet,
			ui:  &params.BandwidthInPeak,
		},
		C.VIR_NETWORK_PORT_BANDWIDTH_IN_BURST: typedParamsFieldInfo{
			set: &params.BandwidthInBurstSet,
			ui:  &params.BandwidthInBurst,
		},
		C.VIR_NETWORK_PORT_BANDWIDTH_IN_FLOOR: typedParamsFieldInfo{
			set: &params.BandwidthInFloorSet,
			ui:  &params.BandwidthInFloor,
		},
		C.VIR_NETWORK_PORT_BANDWIDTH_OUT_AVERAGE: typedParamsFieldInfo{
			set: &params.BandwidthOutAverageSet,
			ui:  &params.BandwidthOutAverage,
		},
		C.VIR_NETWORK_PORT_BANDWIDTH_OUT_PEAK: typedParamsFieldInfo{
			set: &params.BandwidthOutPeakSet,
			ui:  &params.BandwidthOutPeak,
		},
		C.VIR_NETWORK_PORT_BANDWIDTH_OUT_BURST: typedParamsFieldInfo{
			set: &params.BandwidthOutBurstSet,
			ui:  &params.BandwidthOutBurst,
		},
	}
}

// See also https://libvirt.org/html/libvirt-libvirt-network.html#virNetworkPortGetParameters
func (d *NetworkPort) GetParameters(flags uint32) (*NetworkPortParameters, error) {
	params := &NetworkPortParameters{}
	info := getNetworkPortParametersFieldInfo(params)

	var cparams C.virTypedParameterPtr
	var cnparams C.int
	var err C.virError
	ret := C.virNetworkPortGetParametersWrapper(d.ptr, &cparams, &cnparams, C.uint(flags), &err)
	if ret == -1 {
		return nil, makeError(&err)
	}

	defer C.virTypedParamsFreeWrapper(cparams, cnparams)

	_, gerr := typedParamsUnpack(cparams, cnparams, info)
	if gerr != nil {
		return nil, gerr
	}

	return params, nil
}

// See also https://libvirt.org/html/libvirt-libvirt-network.html#virNetworkPortSetParameters
func (d *NetworkPort) SetParameters(params *NetworkPortParameters, flags uint32) error {
	info := getNetworkPortParametersFieldInfo(params)

	cparams, cnparams, gerr := typedParamsPackNew(info)
	if gerr != nil {
		return gerr
	}
	defer C.virTypedParamsFreeWrapper(cparams, cnparams)

	var err C.virError
	ret := C.virNetworkPortSetParametersWrapper(d.ptr, cparams, cnparams, C.uint(flags), &err)
	if ret == -1 {
		return makeError(&err)
	}

	return nil
}
