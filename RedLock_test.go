package RedLock

import (
	"io/ioutil"
	"testing"
	"github.com/garyburd/redigo/redis"
	"log"
	"encoding/json"
	"fmt"
)
func init()  {
	config,err := ioutil.ReadFile("cfg.json")
	if err != nil{
		log.Fatal("read config err",err.Error())
	}
	err = json.Unmarshal(config,&RedCfgParm)
	if err != nil{
		log.Fatal("parse config file failed, ", err.Error())
	}
	fmt.Println("RedCfgParm:",RedCfgParm)
	InitRedisCluster()
}
func InitRedisCluster()  {
	for i,_ := range RedCfgParm.RedisServerList{
		var redisURI string
		redisURI = fmt.Sprintf("%s:%d", RedCfgParm.RedisServerList[i].RedisHost, RedCfgParm.RedisServerList[i].RedisPort)
		fmt.Println("redisURI:",redisURI)
		redisPool := &redis.Pool{
			Dial: func() ( redis.Conn,  error) {
				c,err := redis.Dial("tcp",redisURI)
				if err != nil{
					log.Fatalf("connect to redis(%s) failed:%s",redisURI,err.Error())
				}
				return c,nil
			},
			TestOnBorrow: nil,
			MaxIdle:      120,
			MaxActive:    1200,
			IdleTimeout:  0,
			Wait:         false,
		}
		RedisList = append(RedisList,redisPool)
	}
	QuoRum = len(RedisList) / 2 + 1
}
func TestRedLock(t *testing.T){
	res := Lock("foo")
	if res != true{
		t.Error("RedLock failed!")
	}
	res = UnLock("foo",Gval)
	if res != true{
		t.Error("RedLock UnLock failed!")
	}
}
