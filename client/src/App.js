import React, { Component } from 'react';
import WsClient from './ws-client';
import './App.css';

class App extends Component {
  componentDidMount(){
    const secret = "xxx123456"
    let wsclient = new WsClient("ws://loaclhost:8080",secret)

    wsclient.login(1,"kai")
  }
  render() {
    return (
      <div>
        <div className="Command">
          <input value="" type="text" className="Command-text"></input>
          <input value="send" type="button"></input>
        </div>
        <ul className="Ul-msg">
          <li>msg</li>
        </ul>
      </div>
    );
  }
}

export default App;
