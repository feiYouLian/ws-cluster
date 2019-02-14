
import md5 from 'md5'
import * as byte from 'byte-data';
import utf8BufferSize from 'utf8-buffer-size';

class WsClient {
    constructor(opt){
        this.cfg = {
            url: opt.url,
            secret:opt.secret,
        }
        this.ws_onopen = function(){}
        this.ws_onclose = function(){}
        this.ws_onmessage = function(){}
    }
    login(id,name){
        let nonce = Date.now()+"";
        let digest = md5(`${id}${nonce}${this.cfg.secret}`)
        let wsurl = `${this.cfg.url}/client?id=${id}&name=${name}&nonce=${nonce}&digest=${digest}`
        
        this.conn = new WebSocket(wsurl)
        this.conn.binaryType = "arraybuffer"

        this.conn.onopen = (evt)=>{
            console.log("websocket open success! event_type: "+ evt.type)
            this.ws_onopen(true)
        }
        this.conn.onmessage = (evt)=>{
            this._onmessage(evt.data)
        }
        this.conn.onclose = (evt)=>{
            this._onclose()
        }
        this.conn.onerror = (evt)=>{
            console.error("websocket open failed! event_type: "+ evt.type)
            this.ws_onopen(false)
        }
    }
    // 接收到消息之后处理
    _onmessage(data){
        console.log(data)
        let buf = new Uint8Array(data)
        console.log(buf)
        let len = byte.unpack(buf,{bits:32},0)
        let id = byte.unpack(buf,{bits:32},4)
        let msgType = byte.unpack(buf,{bits:8},8)
        let Scope = byte.unpack(buf,{bits:8},9)
        console.log(len,id,msgType,Scope)
    }
    // 连接关闭
    _onclose(){
        
    }
    onOpen(method){
        this.ws_onopen = method
    }
    onClose(method){
        this.ws_onclose = method
    }
    onMessage(method){
        this.ws_onmessage = method
    }
}

export default WsClient