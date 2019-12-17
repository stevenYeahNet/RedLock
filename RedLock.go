package RedLock

import (
	"fmt"
	"github.com/garyburd/redigo/redis"
	"math"
	"math/rand"
	"os"
	"time"
	"./logging"
)

var unLockScript = redis.NewScript(1,"if redis.call('get', KEYS[1]) == ARGV[1] " +
	"then return redis.call('del', KEYS[1]) else return 0 end")

var continueLockScript = redis.NewScript(1,"if redis.call('get', KEYS[1]) == ARGV[1] then redis.call('del', KEYS[1]) end " +
	"return redis.call('set', KEYS[1], ARGV[2], 'px', ARGV[3], 'nx')")

type RedisServer struct {
	RedisHost	string
	RedisPort	int32
}

type RedCfg struct {
	RedisServerList []RedisServer
	TTl		int32
	RetryCount int32
	RetryDelay int
	ClockDriftFactor float32
}

var RedCfgParm RedCfg
var RedisList []*redis.Pool
var Resource string		//
var Gval	string
var QuoRum int
var ClockDriftFactor = 0.01;	//
/*
 在Redis的分布式环境中，我们假设有N个Redis master。
 这些节点完全互相独立，不存在主从复制或者其他集群协调机制。
*/
/*获取当前Unix时间，以毫秒为单位。
依次尝试从5个实例，使用相同的key和具有唯一性的value（例如UUID）获取锁。当向Redis请求获取锁时，
客户端应该设置一个网络连接和响应超时时间，这个超时时间应该小于锁的失效时间。例如你的锁自动失效时间为10秒，
则超时时间应该在5-50毫秒之间。这样可以避免服务器端Redis已经挂掉的情况下，客户端还在死死地等待响应结果。如果服务器端没有在规定时间内响应，
客户端应该尽快尝试去另外一个Redis实例请求获取锁。客户端使用当前时间减去开始获取锁时间（步骤1记录的时间）就得到获取锁使用的时间。
当且仅当从大多数（N/2+1，这里是3个节点）的Redis节点都取到锁，并且使用的时间小于锁失效时间时，锁才算获取成功。如果取到了锁，
key的真正有效时间等于有效时间减去获取锁所使用的时间（步骤3计算的结果）。如果因为某些原因，获取锁失败（没有在至少N/2+1个Redis实例取到锁或者取锁时间已经超过了有效时间），
客户端应该在所有的Redis实例上进行解锁（即便某些Redis实例根本就没有加锁成功，防止某些节点获取到锁但是客户端没有得到响应而导致接下来的一段时间不能被重新获取锁）*/
//保证原子性（redis是单线程），避免del删除了，其他client获得的lock


func Lock(resource string) bool {
	fmt.Println("Lock")
	if len(Resource) == 0 {
		Resource = resource
		Gval = string(GetUniqueLockId())

	}
	fmt.Println("Gval:",Gval,RedCfgParm.RetryCount)
	for{
		if RedCfgParm.RetryCount <= 0{
			break
		}
		var cnt int = 0
		startTime := time.Now().UnixNano() / int64(time.Millisecond)
		for i := 0; i < len(RedisList); i++{
			if LockInstance(RedisList[i],Resource,Gval,RedCfgParm.TTl){
				cnt++
			}
		}
		drift := int32(RedCfgParm.ClockDriftFactor * float32(RedCfgParm.TTl) + 2)
		endTime := time.Now().UnixNano() / int64(time.Millisecond)
		validityTime := int64(RedCfgParm.TTl) - (endTime - startTime) - int64(drift)
		logging.Log(fmt.Sprintf("The resource validty time is %d, n is %d, quo is %d\n",
			validityTime, cnt, QuoRum))
		if cnt >= QuoRum && validityTime > 0 {
			return true
		}else {
			//unlock

		}
		// Wait a random delay before to retry
		rand.Seed(time.Now().UnixNano())
		delay := rand.Int() % RedCfgParm.RetryDelay + int( math.Floor(float64(RedCfgParm.RetryDelay / 2)) )
		time.Sleep(time.Duration(delay))
		RedCfgParm.RetryCount--
	}
	return false
}

func LockInstance(conn *redis.Pool,resouce,val string,ttl int32)  bool{
	reply,err := conn.Get().Do("set",resouce,val,"px",fmt.Sprintf("%d",ttl),"nx")
	if err != nil{
		fmt.Printf("set %s %s px %d nx failed:%s\n",resouce,val,ttl,err.Error())
		return  false
	}
	if reply.(string) != "OK"{
		return false
	}
	return true
}
func ContinueLockInstance(conn *redis.Pool,resouce,val string,ttl int32) bool {
	_,err := continueLockScript.Do(conn.Get(),resouce,val,ttl)
	if err != nil{
		fmt.Printf("continue redlock(%s,%s) failed:%s",resouce,val,err.Error())
		return false
	}
	return true
}
func UnLockInstance(conn *redis.Pool,resouce,val string) bool {
	_,err := unLockScript.Do(conn.Get(),resouce,val)
	defer conn.Close()
	if err != nil{
		logging.Log( fmt.Sprintf("del redlock(%s--%s) failed:%s\n",resouce,val,err.Error()) )
		return false
	}
	return true
}
func UnLock(resource,val string) bool {
	for idx,_ := range RedisList{
		UnLockInstance(RedisList[idx],resource,val)
	}
	return true
}
func ContinueLock(conn *redis.Pool,resource string,ttl int32) bool {
	if len(Resource) == 0 {
		Resource = resource
		Gval = string(GetUniqueLockId())
		fmt.Println("Gval:",Gval)
	}
	for{
		if RedCfgParm.RetryCount <= 0{
			break
		}
		var cnt int = 0
		startTime := time.Now().UnixNano() / int64(time.Millisecond)
		for i := 0; i < len(RedisList); i++{
			if ContinueLockInstance(RedisList[i],Resource,Gval,ttl){
				cnt++
			}
		}
		drift := int32(RedCfgParm.ClockDriftFactor * float32(ttl) + 2)
		endTime := time.Now().UnixNano() / int64(time.Millisecond)
		validityTime := int64(ttl) - (endTime - startTime) - int64(drift)
		logging.Log(fmt.Sprintf("The resource validty time is %d, n is %d, quo is %d\n",
			validityTime, cnt, QuoRum))
		if cnt >= QuoRum && validityTime > 0 {
			return true
		}else {
			//unlock

		}
		// Wait a random delay before to retry
		rand.Seed(time.Now().UnixNano())
		delay := rand.Int() % RedCfgParm.RetryDelay + int( math.Floor(float64(RedCfgParm.RetryDelay / 2)) )
		time.Sleep(time.Duration(delay))
		RedCfgParm.RetryCount--
	}
	return false
}

func GetUniqueLockId() string {
	file,err := os.OpenFile("/dev/urandom",os.O_RDONLY,0666)
	defer file.Close()
	if err != nil{
		logging.Log( fmt.Sprintln("open /dev/urandom failed:" + err.Error()) )
		return ""
	}
	data := make([]byte,20)
	file.Read(data)
	var id string
	for i := 0; i < 20; i++{
		x := fmt.Sprintf("%02x",data[i])
		id += x
	}
	fmt.Println("id:",id)
	return id
}