package logic

import (
	"../dao"
	"../protocol"
	sev "../server"
	proto "code.google.com/p/goprotobuf/proto"
	log "code.google.com/p/log4go"
)

func processCreateUser(server *sev.Nexus, ipStr string, message *protocol.MobileSuiteModel) {

	createUserDto := &protocol.CreateUserDTO{}
	proto.Unmarshal(message.Message, createUserDto)

	user := &dao.User{}
	user.Name = *createUserDto.Name
	user.Round = 0
	user.WinCount = 0
	user.Rank = 0

	result, err := server.DbConnector.Insert(user)
	if err != nil {
		log.Info(err)
		return
	}
	createResult := &protocol.CreateResultDTO{
		UserId: proto.Int64(int64(result)),
	}
	log.Info(result)
	byt, _ := proto.Marshal(createResult)

	lpDtoMsg := &protocol.MobileSuiteModel{
		Type:    proto.Int32(int32(protocol.MessageType_MSG_TYPE_CREATE_USER_RES)),
		Message: byt,
	}
	server.SendTo(ipStr, lpDtoMsg)
	log.Info(createUserDto.Name)

}

func procerssLogin(server *sev.Nexus, ipStr string, message *protocol.MobileSuiteModel) {
	to := inGameMap[ipStr]
	loginDto := &protocol.LoginDTO{}
	proto.Unmarshal(message.Message, loginDto)

	userArr, _ := server.DbConnector.QueryByUserId(*loginDto.UserId)
	loginResultDto := &protocol.LoginResultDTO{}
	if len(userArr) > 0 {
		user := dao.User(userArr[0])
		// 登陆成功
		loginResultDto.UserId = proto.Int64(user.Id)
		loginResultDto.Name = &user.Name
		loginResultDto.Round = proto.Int32(int32(user.Round))
		loginResultDto.WinCount = proto.Int32(int32(user.WinCount))
		loginResultDto.Rank = proto.Int32(int32(user.Rank))
	}

	byt, _ := proto.Marshal(loginResultDto)
	lpDtoMsg := &protocol.MobileSuiteModel{
		Type:    proto.Int32(int32(protocol.MessageType_MSG_TYPE_LOGIN_RES)),
		Message: byt,
	}

	server.SendTo(to, lpDtoMsg)

}
