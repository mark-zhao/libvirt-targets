package main

import (
	"fmt"
	"strconv"
	"time"

	"metrics/influxdb"

	"github.com/golang/glog"
	"libvirt.org/go/libvirt"
)

func main() {
	var Ms []*influxdb.M
	// vName := "instance-0000000c"
	conn, err := libvirt.NewConnect("qemu:///system")
	if err != nil {
		glog.Info("err", err)
		return
	}
	defer conn.Close()

	// 通过虚拟机名称获取其全部状态
	//conn.ListAllDomains()
	// dob, err := conn.LookupDomainByName(vName)
	// if err != nil {
	// 	fmt.Println("err", err)
	// 	return
	// }
	// dobs := make([]*libvirt.Domain, 0)
	// dobs = append(dobs, dob)
	// dstats, err := conn.GetAllDomainStats(dobs, 0x3FE, libvirt.CONNECT_GET_ALL_DOMAINS_STATS_ACTIVE)
	// if err != nil {
	// 	fmt.Println("err", err)
	// 	return
	// }
	// fmt.Println("dstats", dstats)

	// 获取所有开启的虚拟机
	doms, err := conn.ListAllDomains(libvirt.CONNECT_LIST_DOMAINS_ACTIVE)
	if err != nil {
		glog.Info("err", err)
	}
	glog.Info("%d running domains:\n", len(doms))
	for _, dom := range doms {
		_, err := dom.GetName()
		if err != nil {
			continue
		}
		//内存利用率
		var memRate float64
		// get tag 4:剩余 & 5:总共
		{
			meminfo, err := dom.MemoryStats(10, 0)
			if err != nil {
				continue
			}
			var total uint64
			var usable uint64
			for _, v := range meminfo {
				if v.Tag == 5 {
					total = v.Val
				} else if v.Tag == 8 {
					usable = v.Val
				}
			}
			memUsable := (float64(usable) / float64(total))
			memRate, _ = strconv.ParseFloat(fmt.Sprintf("%.2f", memUsable), 64)
		}
		//cpu 使用率
		var cpu_util float64
		{
			t1 := time.Now().Unix()
			info1, err := dom.GetInfo()
			if err != nil {
				continue
			}
			totalNum := info1.NrVirtCpu
			c1 := info1.CpuTime
			time.Sleep(1 * time.Second)
			t2 := time.Now().Unix()
			info2, err := dom.GetInfo()
			if err != nil {
				continue
			}
			c2 := info2.CpuTime
			usage := float64(c2-c1) / float64(uint64(t2-t1)*uint64(totalNum)*10000000)
			cpu_util, _ = strconv.ParseFloat(fmt.Sprintf("%.2f", usage), 64)
		}
		if uuid, err := dom.GetUUIDString(); err == nil {
			b := influxdb.M{uuid, memRate, cpu_util}
			Ms = append(Ms, &b)
		}

		dom.Free()
	}
	influxdb.InsertInfluxdb(Ms)
}
	