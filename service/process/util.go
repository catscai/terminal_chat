package process

import (
	"fmt"
	"github.com/catscai/ccat/clog"
	"github.com/catscai/terminal_chat/pb/src/allpb"
	"github.com/golang/protobuf/proto"
	"github.com/syndtr/goleveldb/leveldb/opt"
	"github.com/syndtr/goleveldb/leveldb/util"
	"go.uber.org/zap"
	"math/rand"
	"strconv"
	"sync"
	"time"
)

func init() {
	rand.Seed(time.Now().UnixNano())
}

func OnInitUtil() {
	// 遍历leveldb 查找已经使用的UID
	r := &util.Range{
		Start: []byte(fmt.Sprintf(DBUserIDKey, UserMin)),
		Limit: []byte(fmt.Sprintf(DBUserInfoKey, UserMax+1)),
	}
	genUidLock.Lock()
	defer genUidLock.Unlock()
	iter := GLevelDB.NewIterator(r, &opt.ReadOptions{})
	defer iter.Release()
	for iter.Next() {
		value := iter.Value()
		id, err := strconv.ParseInt(string(value), 10, 64)
		if err != nil {
			continue
		}
		genUidMap[id] = struct{}{}
	}
}

const (
	DBUserInfoKey = "user:info:%d"
	DBUserIDKey   = "e:u:%d"
	UserMin       = 100000
	UserMax       = 999999
	GroupMin      = 1000
	GroupMax      = 9999
)

var genUidLock sync.RWMutex

var genUidMap map[int64]struct{} // 已经被使用的uid

var genGroupLock sync.Mutex
var genGroupMap map[int64]struct{} // 已经被使用的group

func init() {
	genUidMap = make(map[int64]struct{})
	genGroupMap = make(map[int64]struct{})
}

func GenUID(logger clog.ICatLog) int64 {
	genUidLock.Lock()
	defer genUidLock.Unlock()
	var curID int64
	for {
		id := rand.Intn(UserMax-UserMin+1) + UserMin
		if _, ok := genUidMap[int64(id)]; !ok {
			curID = int64(id)
			genUidMap[curID] = struct{}{}
			break
		}
	}
	key := fmt.Sprintf(DBUserIDKey, curID)

	err := GLevelDB.Put([]byte(key), []byte(strconv.FormatInt(curID, 10)), &opt.WriteOptions{})
	if err != nil {
		logger.Error("GenUID leveldb set failed", zap.Error(err), zap.Int64("curID", curID))
		return -1
	}

	return curID
}

func GenGroup() int64 {
	genGroupLock.Lock()
	defer genGroupLock.Unlock()
	var curID int64
	for {
		id := rand.Intn(GroupMax-GroupMin+1) + GroupMin
		if _, ok := genGroupMap[int64(id)]; !ok {
			curID = int64(id)
			genGroupMap[curID] = struct{}{}
			break
		}
	}

	return curID
}

func SaveUserInfo(logger clog.ICatLog, info *allpb.UserInfo) error {
	const funcName = "SaveUserInfo"
	key := fmt.Sprintf(DBUserInfoKey, info.GetOwn())
	data, err := proto.Marshal(info)
	if err != nil {
		logger.Error(funcName+" proto.Marshal failed", zap.Error(err), zap.Any("info", info))
		return err
	}
	return GLevelDB.Put([]byte(key), data, &opt.WriteOptions{})
}

func GetUserInfo(logger clog.ICatLog, own int64) (*allpb.UserInfo, error) {
	const funcName = "GetUserInfo"
	key := fmt.Sprintf(DBUserInfoKey, own)
	data, err := GLevelDB.Get([]byte(key), &opt.ReadOptions{})
	if err != nil {
		logger.Error(funcName+" leveldb get failed", zap.Error(err), zap.Int64("own", own))
		return nil, err
	}
	info := &allpb.UserInfo{}
	if err = proto.Unmarshal(data, info); err != nil {
		logger.Error(funcName+" proto.Unmarshal failed", zap.Error(err), zap.Int("data.size", len(data)))
		return nil, err
	}

	return info, nil
}
