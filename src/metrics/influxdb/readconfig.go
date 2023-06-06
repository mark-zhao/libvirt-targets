package influxdb

import (
	"encoding/json"
	"io/ioutil"

	"github.com/golang/glog"
)

//配置文件结构
type DB struct {
	DBip          string `json:"dbip"`
	MyDB          string `json:"mydb"`
	Tags          string `json:"tags"`
	MyMeasurement string `json:"mymeasurement"`
	Username      string `json:"username"`
	Password      string `json:"password"`
}
type Config struct {
	DBInfo DB `json:"db"`
}

// json读取
type JsonStruct struct {
}

func Getconfig() *Config {
	JsonParse := NewJsonStruct()
	v := new(Config)
	JsonParse.Load("./config.json", &v)
	return v
}

//读json 文件
func (jst *JsonStruct) Load(filename string, v interface{}) {
	defer glog.Flush()
	//ReadFile函数会读取文件的全部内容，并将结果以[]byte类型返回
	data, err := ioutil.ReadFile(filename)
	if err != nil {
		glog.Info("error:", err)
		return
	}
	//读取的数据为json格式，需要进行解码
	err = json.Unmarshal(data, v)
	if err != nil {
		glog.Info("error:", err)
		return
	}
}
func NewJsonStruct() *JsonStruct {
	return &JsonStruct{}
}
