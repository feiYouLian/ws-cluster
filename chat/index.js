import ws from "./ws-client";

// 密钥
const secret = "xxx123456"
let wsclient = new ws.WsClient({ url: "ws://192.168.0.127:8380", secret })

wsclient.onOpen = (islogin)=>{
    console.log(islogin?"登陆成功":"登陆失败")
}

wsclient.onMessage = (msg)=>{
    console.log(msg)
}

// 登陆
wsclient.login("111")

// 加入notify组。可以接收发到这些群的消息
wsclient.joinGroups(["notify"])

// 发送消息给单个用户 222
const msgTextType = 1
wsclient.sendToClient("222", msgTextType, "hello")

// 发送消息给群notify
wsclient.sendToGroup("notify", msgTextType, "hello")

// 离开群
wsclient.leaveGroups(["notify"])

setTimeout(()=>{
    // 退出 。退出后会自动退出群
    wsclient.close()
},10000)
