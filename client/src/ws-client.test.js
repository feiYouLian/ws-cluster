import ws from './ws-client';
import ByteBuffer from 'byte';
import { equal, deepEqual } from 'assert';

it('message header', () => {
    let header = new ws.MessageHeader(1, ws.MsgTypeConst.Chat, ws.ScopeConst.Chat, 2)

    let buf = header.encode()
    let wantBytes = new Uint8Array([1, 0, 0, 0, 3, 1, 2, 0, 0, 0])
    deepEqual(buf, wantBytes)
    console.log(wantBytes)

    let header2 = new ws.MessageHeader()
    header2.decode(wantBytes)

    deepEqual(header2, header)
});