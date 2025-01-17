package service

import (
	"fmt"
	"log"
	"net"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"time"
	"x-ui/logger"
	"x-ui/util/common"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

//This should be global variable,and only one instance
var botInstace *tgbotapi.BotAPI

//结构体类型大写表示可以被其他包访问
type TelegramService struct {
	xrayService    XrayService
	inboundService InboundService
	settingService SettingService
}

func (s *TelegramService) GetsystemStatus() string {
	var status string
	//get hostname
	name, err := os.Hostname()
	if err != nil {
		fmt.Println("get hostname error:", err)
		return ""
	}
	status = fmt.Sprintf("主机名称:%s\r\n", name)
	status += fmt.Sprintf("系统类型:%s\r\n", runtime.GOOS)
	status += fmt.Sprintf("系统架构:%s\r\n", runtime.GOARCH)
	//system run time
	systemRuntime, error := exec.Command("bash", "-c", "uptime| sed s/[[:space:]]//g").Output()
	if error != nil {
		logger.Warning("GetsystemStatus error:", err)
	}
	systemStatusStr := common.ByteToString(systemRuntime)
	logger.Info("systemStatusStr:", systemStatusStr)
	status += fmt.Sprintf("运行时间:%s\r\n", strings.Split(systemStatusStr, ",")[0])
	status += fmt.Sprintf("系统负载:%s\r\n", strings.Split(systemStatusStr, ",")[3:])
	//ip address
	var ip string
	netInterfaces, err := net.Interfaces()
	if err != nil {
		fmt.Println("net.Interfaces failed, err:", err.Error())
	}

	for i := 0; i < len(netInterfaces); i++ {
		if (netInterfaces[i].Flags & net.FlagUp) != 0 {
			addrs, _ := netInterfaces[i].Addrs()

			for _, address := range addrs {
				if ipnet, ok := address.(*net.IPNet); ok && !ipnet.IP.IsLoopback() {
					if ipnet.IP.To4() != nil {
						ip = ipnet.IP.String()
						break
					} else {
						ip = ipnet.IP.String()
						break
					}
				}
			}
		}
	}
	status += fmt.Sprintf("IP地址:%s\r\n \r\n", ip)
	//get traffic
	inbouds, err := s.inboundService.GetAllInbounds()
	if err != nil {
		logger.Warning("StatsNotifyJob run error:", err)
	}
	for _, inbound := range inbouds {
		status += fmt.Sprintf("节点名称:%s\r\n端口:%d\r\n上行流量↑:%s\r\n下行流量↓:%s\r\n总流量:%s\r\n", inbound.Remark, inbound.Port, common.FormatTraffic(inbound.Up), common.FormatTraffic(inbound.Down), common.FormatTraffic((inbound.Up + inbound.Down)))
		if inbound.ExpiryTime == 0 {
			status += fmt.Sprintf("到期时间:无限期\r\n \r\n")
		} else {
			status += fmt.Sprintf("到期时间:%s\r\n \r\n", time.Unix((inbound.ExpiryTime/1000), 0).Format("2006-01-02 15:04:05"))
		}
	}
	return status
}

func (s *TelegramService) StartRun() {
	logger.Info("telegram service ready to run")
	s.settingService = SettingService{}
	tgBottoken, err := s.settingService.GetTgBotToken()
	if err != nil || tgBottoken == "" {
		logger.Infof("telegram service start run failed,GetTgBotToken fail,err:%v,tgBottoken:%s", err, tgBottoken)
		return
	}
	logger.Infof("TelegramService GetTgBotToken:%s", tgBottoken)
	botInstace, err = tgbotapi.NewBotAPI(tgBottoken)
	if err != nil {
		logger.Infof("telegram service start run failed,NewBotAPI fail:%v,tgBottoken:%s", err, tgBottoken)
	}
	botInstace.Debug = true
	fmt.Printf("Authorized on account %s", botInstace.Self.UserName)
	//get all my commands
	commands, err := botInstace.GetMyCommands()
	if err != nil {
		logger.Warning("telegram service start run error,GetMyCommandsfail:", err)
	}
	for _, command := range commands {
		fmt.Printf("command %s,Description:%s \r\n", command.Command, command.Description)
	}
	//get update
	chanMessage := tgbotapi.NewUpdate(0)
	chanMessage.Timeout = 60

	updates := botInstace.GetUpdatesChan(chanMessage)

	for update := range updates {
		if update.Message == nil {
			continue
		}

		if !update.Message.IsCommand() {
			continue
		}

		msg := tgbotapi.NewMessage(update.Message.Chat.ID, "")

		// Extract the command from the Message.
		switch update.Message.Command() {
		case "delete":
			inboundPortStr := update.Message.CommandArguments()
			inboundPortValue, err := strconv.Atoi(inboundPortStr)
			if err != nil {
				msg.Text = "Invalid inbound port,please check it"
			}
			//logger.Infof("Will delete port:%d inbound", inboundPortValue)
			error := s.inboundService.DelInboundByPort(inboundPortValue)
			if error != nil {
				msg.Text = fmt.Sprintf("delete inbound whoes port is %d failed", inboundPortValue)
			} else {
				msg.Text = fmt.Sprintf("delete inbound whoes port is %d success", inboundPortValue)
			}
		case "restart":
			err := s.xrayService.RestartXray(true)
			if err != nil {
				msg.Text = fmt.Sprintln("Restart xray failed,error:", err)
			} else {
				msg.Text = "Restart xray success"
			}
		case "disable":
			inboundPortStr := update.Message.CommandArguments()
			inboundPortValue, err := strconv.Atoi(inboundPortStr)
			if err != nil {
				msg.Text = "Invalid inbound port,please check it"
			}
			//logger.Infof("Will delete port:%d inbound", inboundPortValue)
			error := s.inboundService.DisableInboundByPort(inboundPortValue)
			if error != nil {
				msg.Text = fmt.Sprintf("disable inbound whoes port is %d failed,err:%s", inboundPortValue, error)
			} else {
				msg.Text = fmt.Sprintf("disable inbound whoes port is %d success", inboundPortValue)
			}
		case "enable":
			inboundPortStr := update.Message.CommandArguments()
			inboundPortValue, err := strconv.Atoi(inboundPortStr)
			if err != nil {
				msg.Text = "Invalid inbound port,please check it"
			}
			//logger.Infof("Will delete port:%d inbound", inboundPortValue)
			error := s.inboundService.EnableInboundByPort(inboundPortValue)
			if error != nil {
				msg.Text = fmt.Sprintf("enable inbound whoes port is %d failed,err:%s", inboundPortValue, error)
			} else {
				msg.Text = fmt.Sprintf("enable inbound whoes port is %d success", inboundPortValue)
			}
		case "status":
			msg.Text = s.GetsystemStatus()
		default:
			//NOTE:here we need string as a new line each one,we should use ``
			msg.Text = `/delete will help you delete inbound according port
/restart will restart xray,this command will not restart x-ui
/status will get current system info
/enable will enable inbound according port
/disable will disable inbound according port
You can input /help to see more commands`
		}

		if _, err := botInstace.Send(msg); err != nil {
			log.Panic(err)
		}
	}

}

/*
func (s *TelegramService) PrepareCommands() {
	Deletecommand := tgbotapi.BotCommand{
		Command:     "DeleteInbound",
		Description: "This command will help you delete one of your Inbound",
	}
	Helpcommand := tgbotapi.BotCommand{
		Command:     "Help",
		Description: "You can use more command by help command",
	}

}
*/

func (s *TelegramService) SendMsgToTgbot(msg string) {
	logger.Info("SendMsgToTgbot entered")
	tgBotid, err := s.settingService.GetTgBotChatId()
	if err != nil {
		logger.Warning("sendMsgToTgbot failed,GetTgBotChatId fail:", err)
		return
	}
	if tgBotid == 0 {
		logger.Warning("sendMsgToTgbot failed,GetTgBotChatId illegal")
		return
	}

	info := tgbotapi.NewMessage(int64(tgBotid), msg)
	if botInstace != nil {
		botInstace.Send(info)
	} else {
		logger.Warning("bot instance is nil")
	}
}

func (s *TelegramService) StopRunAndClose() {
	if botInstace != nil {
		botInstace.StopReceivingUpdates()
	}
}
