
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
            this.ws_onopen.call(this,true)
        }
        this.conn.onmessage = (evt) => {
            this._onmessage(evt.data)
        }
        this.conn.onclose = (evt) => {
            this._onclose()
        }
        this.conn.onerror = (evt) => {
            console.error("websocket open failed! event_type: " + evt.type)
            this.ws_onopen.call(this,false)
        }
    }
    // 接收到消息之后处理
    _onmessage(data) {
        let arr = new Uint8Array(data)
        let len = arr.length;
        for (let i = 0; i < len;) {
            let len = byte.unpack(arr, { bits: 32 }, i)
            let packet = arr.subarray(i, len)
            let message = MsgUtils.decode(packet)
            this.ws_onmessage.call(this, message)
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
    Ack: 1,  //  消息ack
    Chat: 3,  //  聊天消息
    GroupInOut: 7, //group in out
}

// ScopeConst 消息发送范围
const ScopeConst = {
    // ScopeChat msg to client
    Chat: 1,
    // ScopeGroup msg to a group
    Group: 3,
}


class MessageHeader {
    constructor(id, msgType, scope, to) {
        this.id = id
        this.msgType = msgType
        this.scope = scope
        this.to = to
    }
    // 解码 Buffer to MessageHeader
    decode(buf) {
        this.id = buf.getUint32()
        this.msgType = buf.getUint8()
        this.scope = buf.getUint8()
        if (this.scope > 0) {
            this.to = buf.getString()
        }
    }
    encode(buf) {
        buf.putUint32(this.id)
        buf.putUint8(this.msgType)
        buf.putUint8(this.scope)
        if (this.scope > 0) {
            buf.putString(this.to)
        }
    }
}

class Message {
    constructor(header) {
        this.header = header
    }
    Header() {
        return this.header
    }
}

// MsgChat 聊天消息体
class MsgChat extends Message {
    /**
     * 
     * @param {*} header 
     * @param {*} from_ your clientid
     * @param {*} type 1- text 2-image
     * @param {*} text 
     */
    constructor(header, from_, type, text) {
        super(header)
        this.from = from_
        this.type = type
        this.text = text
    }
    /**
     * return buf下一个位置
     * @param {*} buf 
     */
    decode(buf) {
        this.from = buf.getString()
        this.type = buf.getUint8()
        this.text = buf.getString()
    }
    encode(buf) {
        buf.putString(this.from)
        buf.putUint8(this.type)
        buf.putString(this.text)
    }
}

// MsgGroupInOut 群 in out
class MsgGroupInOut extends Message {
    constructor(header, inout, groups) {
        super(header)
        this.inout = inout
        this.groups = groups
    }
    /**
     * return buf下一个位置
     * @param {*} buf 
     */
    decode(buf) {
        this.inout = buf.getUint8()
        this.groups = buf.getString().split(",")
    }
    encode(buf) {
        buf.putUint8(this.inout)
        buf.putString(this.groups.join(","))
    }
}

class MsgAck extends Message {
    constructor(header, state, desc) {
        super(header)
        this.state = state
        this.desc = desc
    }
    /**
     * return buf下一个位置
     * @param {*} buf 
     */
    decode(buf) {
        this.state = buf.getUint8()
        this.desc = buf.getString()
    }
    encode(buf) {
        buf.putUint8(this.state)
        buf.putString(this.desc)
    }
}

// 字节缓冲，最大1024,只能单向 get 或者 put. 否则 buffer 超过长度
class Buffer {
    BufferSize = 1024;
    constructor(uint8array) {
        if (uint8array) {
            this.buffer = uint8array
            this.length = uint8array.length
        } else {
            this.buffer = new Uint8Array(this.BufferSize)
            this.length = 0
        }
        this._offset = 0 //写偏移
        this._index = 0 //读偏移
    }
    putUint8(value) {
        this._offset = byte.packTo(value, { bits: 8 }, this.buffer, this._offset)
        this.length += 1
    }
    putUint32(value) {
        this._offset = byte.packTo(value, { bits: 32 }, this.buffer, this._offset)
        this.length += 4
    }
    putString(value) {
        let strLen = utf8BufferSize(value)
        this.putUint32(strLen)
        this._offset = byte.packStringTo(value, this.buffer, this._offset)
        this.length += strLen
    }
    getUint8() {
        let value = byte.unpack(this.buffer, { bits: 8 }, this._index)
        this._index += 1
        return value
    }
    getUint32() {
        let value = byte.unpack(this.buffer, { bits: 32 }, this._index)
        this._index += 4
        return value
    }
    getString() {
        let strLen = this.getUint32()
        let value = byte.unpackString(this.buffer, this._index, this._index + strLen)
        this._index += strLen
        return value
    }
    clear() {
        this.buffer = new Uint8Array(this.BufferSize)
        this.length = 0
        this._index = 0
        this._offset = 0
    }
    getBytes() {
        return this.buffer.subarray(0, this.length)
    }
    toString() {
        return this.bytes()
    }
}

const MsgUtils = {
    /**
     * 解码uint8array，返回一个消息对象
     */
    decode: (uint8array) => {
        let buf = new Buffer(uint8array)
        let header = new MessageHeader()
        header.decode(buf)
        let message = this.makeEmptyMsg(header)
        message.decode(buf)
        return message
    },
    /**
     * 编码message，返回一个 UInt8Array
     */
    encode: (message) => {
        let buf = new Buffer()
        message.Header().encode(buf)
        message.encode(buf)
        return buf.getBytes()
    },
    makeEmptyMsg: (header) => {
        switch (header.msgType) {
            case MsgTypeConst.Ack:
                return new MsgAck(header);
            case MsgTypeConst.Chat:
                return new MsgChat(header);
            case MsgTypeConst.GroupInOut:
                return new MsgGroupInOut(header);
            default:
                return null;
        }
    }
}

export default {
    WsClient,
    MessageHeader,
    MsgChat,
    ScopeConst,
    MsgTypeConst,
    Buffer,
}