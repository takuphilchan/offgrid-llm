import {readNativeMessages,writeNativeMessage} from '../../native-message-framing.mjs';
import {attachmentMessage} from '../../attachment-protocol.mjs';

// A diagnostic subprocess, never a registered native-messaging host.
try {
  for await(const incoming of readNativeMessages(process.stdin)){
    const message=attachmentMessage(incoming,'extension');
    if(message.type!=='hello')throw Error('unsupported_fixture_message');
    await writeNativeMessage(process.stdout,attachmentMessage({
      protocol_version:1,type:'hello_ack',connection_id:'fixture-connection',
      client_nonce:message.client_nonce,host_nonce:'b'.repeat(64),
    },'host'));
  }
}catch(error){
  process.stderr.write(['native_message_invalid','native_message_oversize','native_message_truncated','attachment_message_invalid'].includes(error.code)?error.code:'fixture_transport_failed');
  process.exitCode=2;
}
