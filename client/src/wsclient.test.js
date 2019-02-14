import WsClient from './ws-client';

it('login', () => {
    const secret = "xxx123456"
    let wsclient = new WsClient("ws://loaclhost:8080",secret)

    wsclient.login(1,"kai")
    
});