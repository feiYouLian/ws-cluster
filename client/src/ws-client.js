
import md5 from 'md5'

class WsClient {
    constructor(url,secret){
        this.cfg = {
            url,
            secret,
        }
    }
    login(id,name){
        let nonce = new Date()*1+"";
        let digest = md5(`${id}${nonce}${this.cfg.secret}`)
        let wsurl = `${this.cfg.url}/client?id=${id}&name=${name}&nonce=${nonce}&digest=${digest}`
        
        this.conn = new WebSocket(wsurl);
        this.conn.binaryType = "blob"
        this.conn.onopen = function(evt){
            console.log(evt)
        }
        this.conn.onmessage = function(evt){
            this._onmessage(evt.data)
        }
        this.conn.onclose = function(evt){
            
        }
        this.conn.onerror = function(evt){
            console.error(evt.data)
        }
    }
    _onmessage(data){
        console.log(data)
    }
    onOpen(method){
        this.ws_onopen = method
    }
    onMessage(method){
        this.ws_onmessage = method
    }
}

export default WsClient