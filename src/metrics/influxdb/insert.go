package influxdb

import (
	"time"

	"github.com/golang/glog"
	"github.com/influxdata/influxdb/client/v2"
)

var config *Config

//api返回数据
type M struct {
	Uuid      string
	MemUsable float64
	Cpu_util  float64
}

//插入influxdb
func InsertInfluxdb(Ms []*M) {
	config = Getconfig()
	defer glog.Flush()
	conn := connInflux()
	defer conn.Close()
	MyDB := config.DBInfo.MyDB
	Tags := map[string]string{"name": config.DBInfo.Tags}
	MyMeasurement := config.DBInfo.MyMeasurement
	bp, err := client.NewBatchPoints(client.BatchPointsConfig{
		Database:  MyDB,
		Precision: "ns",
	})
	if err != nil {
		glog.Error("error", err)
	}
	//数据录入
	h, _ := time.ParseDuration("-1h")
	for _, v := range Ms {
		fields := map[string]interface{}{
			"MemUsable": v.MemUsable,
			"Cpu_util":  v.Cpu_util,
		}
		// the_time, err := time.Parse("2006-01-02 15:04:05", v.timestamp).Add(8 * h)
		// now := time.Now().Format("2006-01-02 15:04:05")
		// the_time, err := time.Parse("2006-01-02 15:04:05", now)
		the_time := time.Now()
		//a := the_time.Unix()
		//glog.Info("timexx:", the_time, "MyMeasurement:", MyMeasurement, "tags:", Tags, "fields:", fields)
		CheckErr(err)
		//pt, err := client.NewPoint(MyMeasurement, Tags, fields, time.Unix(a, 0))
		// pt, err := client.NewPoint(MyMeasurement, Tags, fields, the_time.Add(8*h))
		Tags["Uuid"] = v.Uuid
		pt, err := client.NewPoint(MyMeasurement, Tags, fields, the_time.UTC())
		CheckErr(err)
		bp.AddPoint(pt)
		if err := conn.Write(bp); err != nil {
			glog.Error("error", err)
		}
		break
	}
}

//建立influxdb链接
func connInflux() client.Client {
	defer glog.Flush()
	username := config.DBInfo.Username
	password := config.DBInfo.Password
	DBip := config.DBInfo.DBip
	cli, err := client.NewHTTPClient(client.HTTPConfig{
		Addr:     "http://" + DBip,
		Username: username,
		Password: password,
	})
	if err != nil {
		glog.Error("error", err)
	}
	return cli
}

func CheckErr(err error) {
	if err != nil {
		glog.Error("error", err)
	}
}
