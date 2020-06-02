# Copyright 2017 The Nuclio Authors.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

import functools
import json
import logging
import os
import tempfile
import threading
import unittest
import base64
import concurrent.futures.thread

import msgpack
import nuclio_sdk
import operator
import sys
import time

import _nuclio_wrapper as wrapper

# python2/3 differences
if sys.version_info[:2] >= (3, 0):
    from socketserver import UnixStreamServer, BaseRequestHandler
    from unittest import mock
else:
    from SocketServer import UnixStreamServer, BaseRequestHandler
    import mock


class TestSubmitEvents(unittest.TestCase):

    def setUp(self):
        self._temp_path = tempfile.mkdtemp(prefix='nuclio-test-py-wrapper')

        # write handler to temp path
        self._handler_path = self._write_handler(self._temp_path)

        # set PYTHONPATH to include temp path
        sys.path.append(self._temp_path)

        # generate socket path
        self._socket_path = os.path.join(self._temp_path, 'nuclio.sock')

        # create transport
        self._unix_stream_server = self._create_unix_stream_server(self._socket_path)

        # create logger
        self._logger = nuclio_sdk.Logger(logging.DEBUG)
        self._logger.set_handler('test-default', sys.stdout, nuclio_sdk.logger.HumanReadableFormatter())

        # create a wrapper
        self._wrapper = wrapper.Wrapper(self._logger, 'reverser:handler', self._socket_path, 'test')

    def tearDown(self):
        sys.path.remove(self._temp_path)
        self._wrapper._processor_sock.close()
        self._unix_stream_server.server_close()
        self._unix_stream_server.shutdown()
        self._unix_stream_server_thread.join()

    def test_event_bigger_than_buffer_size(self):
        self._wrapper._max_buffer_size = 1024
        self._wrapper._unpacker = msgpack.Unpacker(raw=False, max_buffer_size=self._wrapper._max_buffer_size)
        dummy_text = self._wrapper._max_buffer_size * 'a'

        # send the event
        threading.Thread(target=self._send_event, args=(nuclio_sdk.Event(body=dummy_text),)).start()

        # buffer gets full, panic (in real life, the processor should restart itself)
        with self.assertRaises(SystemExit):
            self._wrapper.serve_requests(num_requests=1)

    # TODO: uncomment while fixing it
    # def test_large_event(self):
    #     self._wrapper._max_buffer_size = 1 * 1024 * 1024  # 10mb
    #     self._wrapper._unpacker = msgpack.Unpacker(raw=False, max_buffer_size=self._wrapper._max_buffer_size)
    #
    #     event = nuclio_sdk.Event(body='')
    #
    #     # fill event body as much as possible
    #     while self._get_packed_event_body_len(event) < self._wrapper._max_buffer_size:
    #         event.body += 'a' * int((self._wrapper._max_buffer_size - self._get_packed_event_body_len(event)) / 2)
    #
    #     # send the event
    #     threading.Thread(target=self._send_event, args=(event,)).start()
    #
    #     # handle one request
    #     self._wrapper.serve_requests(num_requests=1)
    #
    #     self._wait_until_received_messages(4)

    def test_single_event(self):
        reverse_text = 'reverse this'

        # send the event
        threading.Thread(target=self._send_event, args=(nuclio_sdk.Event(_id=1, body=reverse_text),)).start()

        self._wrapper.serve_requests(num_requests=1)

        # processor start, function log line, response body, duration messages
        self._wait_until_received_messages(4)

        # extract the response
        response = next(message['body']
                        for message in self._unix_stream_server._messages
                        if message['type'] == 'r')
        response_body = response['body'][::-1]

        if sys.version_info[:2] < (3, 0):

            # blame is on nuclio_sdk/event.py:80
            response_body = base64.b64decode(response_body)

        self.assertEqual(reverse_text, response_body)

    def test_blast_events(self):
        """Test when many >> 10 events are being sent in parallel"""

        def record_event(recorded_events, ctx, event):
            recorded_events.add(event.id)

        recorded_event_ids = set()
        expected_events_length = 100
        self._send_events(expected_events_length)

        # record incoming events
        self._wrapper._entrypoint = functools.partial(record_event, recorded_event_ids)
        self._wrapper.serve_requests(num_requests=expected_events_length)
        self.assertEqual(expected_events_length, len(recorded_event_ids), 'Wrong number of events')

    def test_multi_event(self):
        """Test when two events fit inside on TCP packet"""
        recorded_events = []

        def event_recorder(ctx, event):
            recorded_events.append(event)
            return 'OK'

        num_of_events = 10
        self._send_events(num_of_events)
        self._wrapper._entrypoint = event_recorder
        self._wrapper.serve_requests(num_of_events)
        self.assertEqual(num_of_events, len(recorded_events), 'wrong number of events')

        for recorded_event_index, recorded_event in enumerate(sorted(recorded_events, key=operator.attrgetter('id'))):
            self.assertEqual(recorded_event_index, recorded_event.id)
            response_body = recorded_event.body

            if sys.version_info[:2] < (3, 0):

                # blame is on nuclio_sdk/event.py:80
                response_body = base64.b64decode(response_body)

            self.assertEqual('e{}'.format(recorded_event_index), response_body)

    def _send_event(self, event):
        self._wait_for_socket_creation()
        # print("sending event id {0} sent".format(event.id))

        # pack exactly as processor or wrapper explodes
        body = msgpack.Packer().pack(self._event_to_dict(event))

        total_sent = 0
        while total_sent < len(body):
            sent = self._unix_stream_server._connection_socket.send(body[total_sent:])
            if sent == 0:
                raise RuntimeError('Socket connection broken')
            total_sent = total_sent + sent
        # print("event id {0} sent".format(event.id))

    def _get_packed_event_body_len(self, event):
        return len(msgpack.Packer().pack(self._event_to_dict(event)))

    def _event_to_dict(self, event):
        return json.loads(event.to_json())

    def _send_events(self, num_of_events):
        events = [
            nuclio_sdk.Event(_id=i, body='e{}'.format(i))
            for i in range(num_of_events)
        ]
        pool = concurrent.futures.thread.ThreadPoolExecutor(max_workers=2)
        for event in events:
            pool.submit(self._send_event, event)

    def _wait_for_socket_creation(self, timeout=10, interval=0.1):

        # wait for socket connection
        while self._unix_stream_server._connection_socket is None and timeout > 0:
            time.sleep(interval)
            timeout -= interval

    def _wait_until_received_messages(self, minimum_messages_length, timeout=10, interval=1):
        while timeout > 0:
            time.sleep(interval)
            current_messages_length = len(self._unix_stream_server._messages)
            if current_messages_length >= minimum_messages_length:
                break
            self._logger.debug_with('Waiting for messages to arrive',
                                    current_messages_length=current_messages_length,
                                    minimum_messages_length=minimum_messages_length)
            timeout -= interval

    def _create_unix_stream_server(self, socket_path):
        unix_stream_server = _SingleConnectionUnixStreamServer(socket_path, _Connection)

        # create a thread and listen forever on server
        self._unix_stream_server_thread = threading.Thread(target=unix_stream_server.serve_forever)
        self._unix_stream_server_thread.daemon = True
        self._unix_stream_server_thread.start()
        return unix_stream_server

    def _write_handler(self, temp_path):
        handler_code = '''import sys

is_py2 = sys.version_info[:2] < (3, 0)

def handler(ctx, event):
    """Return reversed body as string"""
    body = event.body
    if not is_py2 and isinstance(body, bytes):
        body = body.decode('utf-8')
    ctx.logger.warn('the end is nigh')
    return body[::-1]
'''

        handler_path = os.path.join(temp_path, 'reverser.py')

        with open(handler_path, 'w') as out:
            out.write(handler_code)

        return handler_path


class _SingleConnectionUnixStreamServer(UnixStreamServer):

    def __init__(self, server_address, RequestHandlerClass, bind_and_activate=True):
        UnixStreamServer.__init__(self, server_address, RequestHandlerClass, bind_and_activate)

        self._connection_socket = None
        self._messages = []


class _Connection(BaseRequestHandler):

    def handle(self):
        self.request.settimeout(1)

        # make a file from the socket so we can readln
        socket_file = self.request.makefile('r')

        # save the connection socket
        self.server._connection_socket = self.request

        # while the server isn't shut down
        while not self.server._BaseServer__shutdown_request:

            try:
                line = socket_file.readline()
                if not line:
                    continue

                message = {
                    'type': line[0],
                    'body': json.loads(line[1:]) if line[0] != 's' else ''
                }

                self.server._messages.append(message)

            except:
                pass


class TestCallFunction(unittest.TestCase):

    def setUp(self):
        # provided by _connection_provider
        self._mockConnection = mock.MagicMock()

    def test_call_json_body(self):
        self._platform = nuclio_sdk.Platform('local', 'somens', self._connection_provider)

        # prepare an event to send
        event = nuclio_sdk.Event(method='GET', path='path', body={'a': 'some_body'})

        # prepare a responder
        connection_response = mock.MagicMock()
        connection_response.status = 204
        connection_response.getheaders = lambda: [('Content-Type', 'application/json')]
        connection_response.read = mock.MagicMock(return_value='{"b": "some_response"}')

        self._mockConnection.getresponse = mock.MagicMock(return_value=connection_response)

        # send the event
        response = self._platform.call_function('function-name', event)

        self.assertEqual(self._mockConnection.url, 'nuclio-somens-function-name:8080')
        self._mockConnection.request.assert_called_with(event.method,
                                                        event.path,
                                                        body=json.dumps({'a': 'some_body'}),
                                                        headers={
                                                            'Content-Type': 'application/json',
                                                            'X-Nuclio-Target': 'function-name'
                                                        })

        self.assertEqual({'b': 'some_response'}, response.body)
        self.assertEqual('application/json', response.content_type)
        self.assertEqual(204, response.status_code)

    def test_get_function_url(self):
        self.assertEqual(nuclio_sdk.Platform('local', 'ns')._get_function_url('function-name'),
                         'nuclio-ns-function-name:8080')
        self.assertEqual(nuclio_sdk.Platform('kube', 'ns')._get_function_url('function-name'),
                         'nuclio-function-name:8080')

    def _connection_provider(self, url, timeout=None):
        self._mockConnection.url = url
        return self._mockConnection
