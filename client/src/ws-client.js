
import md5 from 'md5'
import * as byte from 'byte-data';
import utf8BufferSize from 'utf8-buffer-size';

let _latest_id = 0
function getId() {
    _latest_id++
    return _latest_id
}

class WsClient {
    constructor(opt) {
        opt = opt || {}
        if (opt.autoConnect === undefined) {
            opt.autoConnect = true
        }
        this.cfg = {
            url: opt.url,
            secret: opt.secret,
            autoConnect: opt.autoConnect,
        }
        this.onOpen = function () { }
        this.onClose = function () { }
        this.onMessage = function () { }
        // 保存节点信息
        this.client = {}
        // 保存当前 client 所在的 group
        this.groups = new Set()
    }
    login(id) {
        id = id + '';
        let nonce = Date.now() + "";
        let digest = md5(`${id}${nonce}${this.cfg.secret}`)
        let wsurl = `${this.cfg.url}/client?id=${id}&nonce=${nonce}&digest=${digest}`

        this.conn = new WebSocket(wsurl)
        this.conn.binaryType = "arraybuffer"

        this.conn.onopen = (evt) => {
            console.log("websocket open success! event_type: " + evt.type)
            this.onOpen(true)
            this.client.id = id
        }
        this.conn.onmessage = (evt) => {
            this._onmessage(evt.data)
        }
        this.conn.onclose = (evt) => {
            this._onclose()
        }
        this.conn.onerror = (evt) => {
            console.error("websocket open failed! event_type: " + evt.type)
            this.onOpen(false)
        }

    }
    // 接收到消息之后处理
    _onmessage(data) {
        let arr = new Uint8Array(data)
        let len = arr.length;
        for (let i = 0; i < len;) {
            let len = byte.unpack(arr, { bits: 32 }, i)
            let packet = arr.subarray(i + 4, len + 4)
            console.log(packet)
            let message = MsgUtils.decode(packet)
            this.onMessage(message)
            i += 4 + len;
        }
    }
    close() {
        this.forceExit = true
        this.client = {}
        this.conn.close()
    }
    // 连接关闭
    _onclose() {
        if (this.forceExit || !this.cfg.autoConnect) {
            if (!this.cfg.autoConnect) {
                this.onClose()
            }
            return true
        }
        setTimeout(() => {
            this.login(this.client.id);
        }, 1000);
    }
    /**
     * 发送消息到一个 client, 
     * @param {*} to 
     * @param {*} type 
     * @param {*} text 
     * @param {*} extra 
     */
    sendToClient(to, type, text, extra) {
        if (!to) {
            throw new Error("to is empty")
        }
        if (!text && !extra && !type) {
            return
        }
        let id = getId()
        let header = new MessageHeader(id, MsgTypeConst.Chat, ScopeConst.Client, to + '')
        let msg = new MsgChat(header, this.client.id, type, text, extra)
        this.send(msg)
        return msg
    }
    sendToGroup(group, type, text, extra) {
        if (!group) {
            throw new Error("to is empty")
        }
        if (!text && !extra && !type) {
            return
        }
        let id = getId()
        let header = new MessageHeader(id, MsgTypeConst.Chat, ScopeConst.Group, group)
        let msg = new MsgChat(header, this.client.id, type, text, extra)
        this.send(msg)
        return msg
    }
    // 发送一个消息对象，此消息对象必须 extends Message
    send(msg) {
        if (!msg) {
            return
        }
        let bytes = MsgUtils.encode(msg)
        console.debug(bytes)
        if (bytes != null && bytes.length > 0)
            this.conn.send(bytes)
    }
    joinGroups(groups) {
        if (groups === undefined || groups.length === undefined || groups.length === 0) {
            console.error("joinGroups: groups no found")
            return
        }

        let header = new MessageHeader(getId(), MsgTypeConst.GroupInOut)
        let msg = new MsgGroupInOut(header, 1, groups)
        this.send(msg)
        groups.forEach(group => {
            this.groups.add(group)
        })
    }
    leaveGroups(groups) {
        if (groups === undefined || groups.length === undefined || groups.length === 0) {
            console.error("joinGroups: groups no found")
            return
        }
        let header = new MessageHeader(getId(), MsgTypeConst.GroupInOut)
        let msg = new MsgGroupInOut(header, 0, groups)
        this.send(msg)
        groups.forEach(group => {
            this.groups.delete(group)
        })
    }
    msgAck(id, to, state, desc) {
        let header = new MessageHeader(id, MsgTypeConst.Ack, ScopeConst.Chat, to)
        let msg = new MsgAck(header, state, desc)
        this.send(msg)
    }
}

// MsgTypeConst 定义了各种消息类型
const MsgTypeConst = {
    Ack: 1,  //  消息ack
    Chat: 3,  //  聊天消息
    GroupInOut: 5, //group in out
}

// ScopeConst 消息发送范围
const ScopeConst = {
    // ScopeChat msg to client
    Client: 1,
    // ScopeGroup msg to a group
    Group: 3,
}


class MessageHeader {
    constructor(id, msgType, scope, to) {
        this.id = id
        this.msgType = msgType
        this.scope = scope || 0
        this.to = to || ""
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
     * @param {*} text 消息正文
     * @param {*} extra 额外信息
     */
    constructor(header, from_, type, text, extra) {
        super(header)
        this.from = from_
        this.type = type || 1
        this.text = text || ""
        this.extra = extra || ""
    }
    /**
     * return buf下一个位置
     * @param {*} buf 
     */
    decode(buf) {
        this.from = buf.getString()
        this.type = buf.getUint8()
        this.text = buf.getString()
        this.extra = buf.getString()
    }
    encode(buf) {
        buf.putString(this.from)
        buf.putUint8(this.type)
        buf.putString(this.text)
        buf.putString(this.extra)
    }
}

// MsgGroupInOut 群 in out
class MsgGroupInOut extends Message {
    /**
     * 
     * @param {*} header 
     * @param {*} inout 0 leave ; 1 join
     * @param {*} groups group array
     */
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
        this.state = state || 0
        this.desc = desc || ""
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
    constructor(uint8array, offset) {
        if (uint8array) {
            this.buffer = uint8array
            this.length = uint8array.length
        } else {
            this.buffer = new Uint8Array(this.BufferSize)
            this.length = offset || 0
        }
        this._offset = offset || 0 //写偏移
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
    setUint32(offset, value) {
        byte.packTo(value, { bits: 32 }, this.buffer, offset)
        if ((offset + 4) > this.length) {
            this.length = offset + 4
        }
    }
    putString(value) {
        let strLen = utf8BufferSize(value)
        this.putUint32(strLen)
        if (strLen > 0) {
            this._offset = byte.packStringTo(value, this.buffer, this._offset)
            this.length += strLen
        }
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
        if (strLen === 0) {
            return ""
        }
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
        try {
            let buf = new Buffer(uint8array)
            let header = new MessageHeader()
            header.decode(buf)
            let message = MsgUtils.makeEmptyMsg(header)
            if (!message) {
                return null
            }
            message.decode(buf)
            return message
        } catch (error) {
            console.error(error)
        }
        return null
    },
    /**
     * 编码message，返回一个 UInt8Array
     */
    encode: (message) => {
        try {
            let buf = new Buffer(null, 4)
            message.Header().encode(buf)
            message.encode(buf)
            // 把消息长度写到前字节
            buf.setUint32(0, buf.length - 4)
            return buf.getBytes()
        } catch (error) {
            console.error(error)
        }
        return []
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
    MsgAck,
    MsgGroupInOut,
}