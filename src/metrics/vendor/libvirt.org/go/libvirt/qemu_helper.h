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
 * Copyright (c) 2013 Alex Zorin
 * Copyright (C) 2016 Red Hat, Inc.
 *
 */

#ifndef LIBVIRT_GO_QEMU_HELPER_H__
#define LIBVIRT_GO_QEMU_HELPER_H__

#include "libvirt_qemu_generated.h"


void
domainQemuMonitorEventCallbackHelper(virConnectPtr conn,
                                     virDomainPtr dom,
                                     const char *event,
                                     long long secs,
                                     unsigned int micros,
                                     const char *details,
                                     void *data);


int
virConnectDomainQemuMonitorEventRegisterHelper(virConnectPtr conn,
                                               virDomainPtr dom,
                                               const char *event,
                                               long goCallbackId,
                                               unsigned int flags,
                                               virErrorPtr err);


#endif /* LIBVIRT_GO_QEMU_HELPER_H__ */
