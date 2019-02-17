import React, { Component } from 'react';
import ws from './ws-client';
import './App.css';

// 应答消息的几个状态
const AckStateMapping = {
  0: "sending",
  1: "fail",
  2: "sent",
  3: "read",
}
class App extends Component {
  constructor(props) {
    super(props)
    this.state = {
      msgs: [],
      login: false,
      scope: 0,
      msg_text: "test",
      clientId: Date.now() + "",  // 生成一个随机 id, 登陆服务器,方便测试
    }
    // 密钥
    const secret = "xxx123456"
    this.wsclient = new ws.WsClient({ url: "ws://localhost:8080", secret })
    this.wsclient.onOpen = this.onOpen.bind(this)
    this.wsclient.onMessage = this.wsOnMessage.bind(this)
  }
  onOpen(login) {
    console.debug(login)
    // 默认登陆状态
    this.setState({ login })
  }
  componentDidMount() {
    this.wsclient.login(this.state.clientId)
  }
  componentWillUnmount() {
    // 退出服务器
    this.wsclient.close()
  }
  wsOnMessage(msg) {
    console.log("read a message", msg)
    let { msgs } = this.state
    if (msg.Header().msgType === ws.MsgTypeConst.Ack) {
      // 找到应应答消息的原消息
      let chatMsg = msgs.find(m => m.Header().id === msg.Header().id);
      if (chatMsg) {
        chatMsg.ackState = msg.state
      }
    } else {
      msgs.push(msg)
    }
    this.setState({ msgs })
  }
  sendMessage(scope, to, text) {
    let { msgs } = this.state

    let msg = this.wsclient.sendToClient(to, scope, text)
    msg.ackState = 0

    msgs.push(msg)
    this.setState({ msgs })
  }
  renderHeader(header) {
    let scope = "";
    if (header.scope > 0) {
      scope = header.scope === ws.ScopeConst.Group ? "group" : "client"
    }
    return (
      <div>
        <label>ID:{header.id} </label>
        <label>msgType:{header.msgType} </label>
        <label>scope:{scope} </label>
        <label>to:{header.to || ""} </label>
      </div>
    )
  }
  renderChatMsg(msg, i) {
    // 应答消息的 ID 与 发送时的 ID 一样
    return (<div key={i} className="msg-li">
      {this.renderHeader(msg.header)}
      <div>
        <label>Text:{msg.text}</label>
        <label>Extra:{msg.extra}</label>
        <label>From:{msg.from === this.wsclient.client.id ? "自己" : msg.from}</label>
        <font color="red">{msg.from !== this.wsclient.client.id ? "" : ("状态：" + AckStateMapping[msg.ackState])}</font>
        {
          msg.from !== this.wsclient.client.id ? (<button onClick={e => {
            // ack message read
            this.wsclient.msgAck(msg.header.id, msg.from, 3, "")
          }}>read it</button>) : ""
        }
      </div>
    </div>)
  }
  render() {
    let { login, msgs, clientId, scope, to, msg_text } = this.state;
    console.log(msgs)
    return (
      <div>
        <div className="Command">
          <select value={scope} onChange={e => {

          }}>
            <option value={ws.ScopeConst.Client}>client</option>
            <option value={ws.ScopeConst.Client}>group</option>
          </select>
          <label>TO:</label>
          <input value={to} type="text" className="Command-text" onChange={e => {
            this.setState({ to: e.target.value });
          }}></input>
          <label>Text:</label>
          <input value={msg_text} type="text" className="Command-text" onChange={e => {
            this.setState({ msg_text: e.target.value });
          }}></input>
          <input value="send" type="button" onClick={e => {
            this.sendMessage(scope, to, msg_text)
          }}></input>
        </div>
        <div>
          clientId:[{clientId}]当前登陆状态： {login ? "已登陆" : "未登陆"}
        </div>
        <div className="Ul-msg">
          {
            msgs.map((msg, i) => {
              if (msg.header.msgType === ws.MsgTypeConst.Chat) {
                return this.renderChatMsg(msg, i)
              }
              return (<div>none</div>)
            }
            )}
        </div>
      </div>
    );
  }
}

export default App;
