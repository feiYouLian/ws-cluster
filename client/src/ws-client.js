
import md5 from 'md5'
import * as byte from 'byte-data';
import utf8BufferSize from 'utf8-buffer-size';

class WsClient {
    constructor(opt) {
        this.cfg = {
            url: opt.url,
            secret: opt.secret,
        }
        this.ws_onopen = function () { }
        this.ws_onclose = function () { }
        this.ws_onmessage = function () { }
    }
    login(id, name) {
        let nonce = Date.now() + "";
        let digest = md5(`${id}${nonce}${this.cfg.secret}`)
        let wsurl = `${this.cfg.url}/client?id=${id}&name=${name}&nonce=${nonce}&digest=${digest}`

        this.conn = new WebSocket(wsurl)
        this.conn.binaryType = "arraybuffer"

        this.conn.onopen = (evt) => {
            console.log("websocket open success! event_type: " + evt.type)
            this.ws_onopen(true)
        }
        this.conn.onmessage = (evt) => {
            this._onmessage(evt.data)
        }
        this.conn.onclose = (evt) => {
            this._onclose()
        }
        this.conn.onerror = (evt) => {
            console.error("websocket open failed! event_type: " + evt.type)
            this.ws_onopen(false)
        }
    }
    // 接收到消息之后处理
    _onmessage(data) {
        let buf = new Uint8Array(data)
        let len = buf.length;
        for (let i = 0; i < len;) {
            let len = byte.unpack(buf, { bits: 32 }, 0)
            let packet = buf.subarray(0, len)
            
            i += len;
        }


    }
    // 连接关闭
    _onclose() {

    }
    onOpen(method) {
        this.ws_onopen = method
    }
    onClose(method) {
        this.ws_onclose = method
    }
    onMessage(method) {
        this.ws_onmessage = method
    }
}

// MsgTypeConst 定义了各种消息类型
const MsgTypeConst = {
    // MsgTypeChat 单聊消息
    Chat: 3,
    // MsgTypeChatAck ack
    ChatAck: 4,
    // MsgTypeGroup group
    ChatGroup: 5,
    // MsgTypeJoinGroup join group
    JoinGroup: 7,
    // MsgTypeLeaveGroup leave group
    LeaveGroup: 9,
}

// ScopeConst 消息发送范围
const ScopeConst = {
    // ScopeChat msg to client
    Chat: 1,
    // ScopeGroup msg to a group
    Group: 3,
}

const BufferSize = 1024;


class MessageHeader {
    constructor(id, msgType, scope, to) {
        this.id = id
        this.msgType = msgType
        this.scope = scope
        this.to = to
    }
    // 解码 Uint8Array to MessageHeader
    decode(buf) {
        this.id = byte.unpack(buf, { bits: 32 }, 0)
        this.msgType = byte.unpack(buf, { bits: 8 }, 4)
        this.scope = byte.unpack(buf, { bits: 8 }, 5)
        if (this.scope > 0) {
            if (this.scope === ScopeConst.Chat) {
                this.to = byte.unpack(buf, { bits: 64  }, 6)
            } else if (this.scope === ScopeConst.Group) {
                let strLen = byte.unpack(buf, { bits: 32 }, 6)
                this.to = byte.unpackString(buf, 10, strLen)
            }
        }
    }
    encode() {
        let d = this;
        let buf = new Uint8Array(100)
        let nindex = byte.packTo(d.id, { bits: 32 }, buf)
        nindex = byte.packTo(d.msgType, { bits: 8 }, buf, nindex)
        nindex = byte.packTo(d.scope, { bits: 8 }, buf, nindex)
        if (d.scope !== 0 && d.to !== undefined) {
            if (d.scope === ScopeConst.Chat) {
                nindex = byte.packTo(d.to, { bits: 64 }, buf, nindex)
            } else if (this.scope === ScopeConst.Group) {
                let strLen = utf8BufferSize(d.to)
                nindex = byte.packTo(strLen, { bits: 32 }, buf, nindex)
                nindex = byte.packStringTo(d.to, buf, nindex)
            }
        }
        return buf.subarray(0, nindex)
    }
}

class Msgchat extends MessageHeader {
    constructor(id, msgType, scope, to, from_, type, text) {
        super(id, msgType, scope, to)
        this.from = from_
        this.type = type
        this.text = text
    }
    decode(buf) {
        this.from = byte.unpack(buf, { bits: 64 }, 0)
        this.type = byte.unpack(buf, { bits: 8 }, 8)
        let strLen = byte.unpack(buf, { bits: 32 }, 9)
        this.text = byte.unpackString(buf, 10, strLen)
    }
    encode() {
        let d = this;
        let buf = new Uint8Array(BufferSize)
        let nindex = byte.packTo(d.from, { bits: 32 }, buf)
        nindex = byte.packTo(d.type, { bits: 8 }, buf, nindex)
        nindex = byte.packStringTo(d.text, buf, nindex)
        return buf.subarray(0, nindex)
    }
}

export default {
    WsClient,
    MessageHeader,
    Msgchat,
    ScopeConst,
    MsgTypeConst,
}