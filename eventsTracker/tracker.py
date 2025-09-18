import random
import socket
import threading
import time
from datetime import datetime
import os
import traceback
import sys
import boto3
import json

sys.setrecursionlimit(3000)

from config import *
# from notify_telegram import notify_error

sys.path.insert(0, os.path.abspath(os.path.join(os.path.dirname(__file__), "../events_to_regions")))


def log_data(timestamp, username, user_id, type_event, sub_tier, sub_months, quantity):
    """Log the data in the events.txt file and call the function to assign new regions to the new events"""
    new_id = -1
    with open("events.txt", "r") as f:
        lines = f.readlines()
        old_id = lines[-1].split("\t")[0] if len(lines) > 0 else 0
        new_id = int(old_id) + 1

    with open("events.txt", "a") as f:
        new_data_str = f"{new_id}\t{timestamp}\t{username}\t{user_id}\t{type_event}\t{sub_tier}\t{sub_months}\t{quantity}\n"

        f.write(new_data_str)


def push_element_to_queue(timestamp, username, user_id, type_event, sub_tier, sub_months, quantity):
    log_data(timestamp, username, user_id, type_event, sub_tier, sub_months, quantity)

    try:
        user_id = int(user_id)
    except (ValueError, TypeError):
        pass

    data = {
        "user_id": user_id,
        "username": username,
        "datetime": datetime.fromtimestamp(timestamp).strftime('%Y-%m-%d %H:%M:%S'),
        "event": {
            "event_type": type_event,
            "user_tier": sub_tier,
            "months": sub_months,
            "n_bits": quantity if type_event == "bits" else None
        }
    }

    # Get the service resource
    sqs = boto3.resource('sqs')

    # Get the queue. This returns an SQS.Queue instance
    queue = sqs.get_queue_by_name(QueueName=SQS_QUEUE_NAME)

    try:
        response = queue.send_message(
            MessageBody=json.dumps(data),
            MessageGroupId="subvision",
            MessageDeduplicationId=str(timestamp) + username + type_event + str(random.randint(0, 1000000))
        )
    except Exception as error:
        # notify telegram
        raise error


def get_data_from_line_privmsg_manual_event(line):
    """Get the data from the line of the privmsg event"""
    timestamp, display_name, is_mod = None, None, None

    # get username
    line_list = line.split(";")
    for e in line_list:
        if "tmi-sent-ts" == e.split("=")[0]:
            timestamp = round(int(e.split("=")[1])/1000)

        if "display-name" == e.split("=")[0]:
            display_name = e.split("=")[1]

        elif "mod=" in e:
            is_mod = e.split("=")[1] == "1"

    if display_name.lower() == CHANNEL.lower():
        is_mod = True

    message = "".join(line.split("PRIVMSG #")[1].split(" :")[1]).strip()
    message_splitted = message.split(" ") 
    if len(message_splitted) < 2 or message_splitted[0] != "!subvision":
        return None, None, None, None

    username = message_splitted[1]

    try:
        user_id = int(message_splitted[2])
    except ValueError:
        user_id = None
    
    return timestamp, username, user_id, is_mod


def get_data_from_line_usernotice(line):
    """Get the data from the line of the usernotice event"""
    timestamp, user_id, username, event_type, quantity = None, None, None, None, None

    sub_months = -1
    sub_tier = ""

    # get event type and username
    line_list = line.split(";")
    for e in line_list:
        if "tmi-sent-ts" == e.split("=")[0]:
            timestamp = round(int(e.split("=")[1])/1000)

        if "msg-id" == e.split("=")[0]:
            event_type = e.split("=")[1]

        if "user-id" == e.split("=")[0]:
            user_id = e.split("=")[1]
        
        if "display-name" == e.split("=")[0]:
            username = e.split("=")[1]

        if event_type == "subgift":
            quantity = 1

        if event_type == "submysterygift" and "msg-param-mass-gift-count" == e.split("=")[0]:
            quantity = int(e.split("=")[1])

        if (event_type == "resub" or event_type == "sub") and "msg-param-cumulative-months" == e.split("=")[0]:
            sub_months = int(e.split("=")[1])

        if (event_type == "resub" or event_type == "sub") and "msg-param-sub-plan" == e.split("=")[0]:
            sub_tier = e.split("=")[1]
            if "Prime" in sub_tier:
                sub_tier = "Prime"
            elif "1000" in sub_tier:
                sub_tier = "Tier1"
            elif "2000" in sub_tier:
                sub_tier = "Tier2"
            elif "3000" in sub_tier:
                sub_tier = "Tier3"

    return timestamp, username, user_id, event_type, sub_tier, sub_months, quantity


def get_data_bits_from_line_privmsg(line):
    """Get the data from the line of the privmsg with bits event"""
    timestamp, username, user_id = None, None, None
    bits = 0

    # get bits and username
    line_list = line.split(";")
    for e in line_list:
        if "tmi-sent-ts" == e.split("=")[0]:
            timestamp = round(int(e.split("=")[1])/1000)

        if "bits=" in e:
            bits = e.split("=")[1]

        if "user-id" == e.split("=")[0]:
            user_id = e.split("=")[1]

        if "display-name" == e.split("=")[0]:
            username = e.split("=")[1]

    # if the bits are not a number, return 0
    try:
        bits = int(bits)
    except ValueError:
        return timestamp, username, 0

    return timestamp, username, user_id, bits


class ListenChatThread(threading.Thread):
    def __init__(self, channel:str):
        super(ListenChatThread, self).__init__()
        self.channel = channel
        self.socket_irc = None
        self.log = ""
        self._reloading_irc_connection = threading.Event()

    def set_reloading_irc_connection(self):
        self._reloading_irc_connection.set()

    def clear_reloading_irc_connection(self):
        self._reloading_irc_connection.clear()

    def is_reloading_irc_connection(self):
        return self._reloading_irc_connection.is_set()

    def reload_irc_connection(self):
        print(f"> Reloading irc connection for {self.channel}")
        self.set_reloading_irc_connection()     # set the flag to avoid the thread to listen the chat while reloading the connection

        time.sleep(5.5)   # wait that the thread pause the listening of the chat

        self.socket_irc.close()

        self.socket_irc = self.connect()    # set the new irc socket

        readbuffer = self.socket_irc.recv(65536).decode()
        count = 0
        while "Login unsuccessful" in readbuffer and count <= 20:
            self.socket_irc.close()
            time.sleep(random.uniform(0.5, 2))    # randomize the sleep to avoid all the threads to reload the connection at the same time
            self.socket_irc = self.connect()
            readbuffer = self.socket_irc.recv(65536).decode()
            count += 1

        if count > 20 and "Login unsuccessful" in readbuffer:
            self.socket_irc.close()
            print(f"> Login unsuccessful during reload irc connection {self.channel}")
            return -1
        
        self.socket_irc.settimeout(10)

        self.clear_reloading_irc_connection()   # clear the flag to allow the thread to listen the chat
        print(f"> Reloaded irc connection for {self.channel}")

    def connect(self):
        # print(f"\t\t CONNECTING TO {self.channel}")
        irc = socket.socket()
        irc.connect((SERVER, 6667)) #connects to the server

        #sends variables for connection to twitch chat
        irc.send(('PASS ' + PASSWORD + '\r\n').encode())
        irc.send(('NICK ' + NICK + '\r\n').encode())

        irc.send(('CAP REQ :twitch.tv/tags\r\n').encode())
        irc.send(('CAP REQ :twitch.tv/commands\r\n').encode())
        irc.send(('raw CAP REQ :twitch.tv/membership\r\n').encode())

        irc.send(('JOIN #' + self.channel + '\r\n').encode())

        return irc

    def start_listen(self):
        print(f"> Start listening to {self.channel}")

        readbuffer = ""
        count_timeout = 0
        while True:
            if self.is_reloading_irc_connection():  # wait for the connection to be ready, used for the reload of the irc connection
                time.sleep(0.1)
                continue

            try:
                readbuffer = self.socket_irc.recv(65536).decode()
                count_timeout = 0
            except socket.timeout:
                count_timeout += 1
            except UnicodeDecodeError:
                # notify_error("UnicodeDecodeError in subvision event tracker")
                continue

            if count_timeout >= 10 and not self.is_reloading_irc_connection():
                # print(f"> Reload irc connection after 10 timeouts (100 seconds) for {self.channel}")
                self.reload_irc_connection()
                count_timeout = 0
                continue

            lines = readbuffer.split("\n")
            for line in lines:
                # print(line)

                if "PRIVMSG" in line and "!subvision" in line:
                    ts, username, user_id, is_mod = get_data_from_line_privmsg_manual_event(line)

                    if username is None or ts is None or user_id is None or not is_mod:
                        continue

                    print(f"Adding manual event for {username}")
                    push_element_to_queue(ts, username, user_id, "manual_event", None, None, 0)

                if "PRIVMSG" in line and "bits=" in line:
                    ts, username, user_id, n_bits = get_data_bits_from_line_privmsg(line)

                    if n_bits >= 100 and username is not None:
                        push_element_to_queue(ts, username, user_id, "bits", None, None, n_bits)

                if "USERNOTICE" in line:
                    ts, username, user_id, event_type, sub_tier, sub_months, quantity = get_data_from_line_usernotice(line)
                    if ts is None or username is None or event_type is None:
                        # send the line to the telegram bot
                        # notify_error(f"Error in subvision events tracker during parsing of line:\n{line}")
                        continue

                    if event_type == "announcement":
                        continue

                    # check that the username does not have another sub or resub event in the last 25 days
                    elif event_type == "sub" or event_type == "resub":
                        push_element_to_queue(ts, username, user_id, event_type, sub_tier, sub_months, quantity)

                    # check that the username does not have a subgift event in the last 15 seconds
                    elif event_type == "submysterygift":
                        check = True
                        with open("events.txt", "r") as f:
                            lines = f.readlines()
                        for l in lines:
                            username_l = l.split("\t")[2].strip()
                            if username == username_l and "subgift" in l:
                                last_subgift = datetime.fromtimestamp(int(l.split("\t")[1]))
                                if (datetime.fromtimestamp(int(ts)) - last_subgift).seconds < 15:
                                    check = False
                                    break

                        if check:
                            push_element_to_queue(ts, username, user_id, event_type, sub_tier, sub_months, quantity)

                    # check that the username does not have a submysterygift event in the last 15 seconds
                    elif event_type == "subgift":
                        check = True
                        with open("events.txt", "r") as f:
                            lines = f.readlines()
                        for l in lines:
                            username_l = l.split("\t")[2].strip()
                            if username == username_l and "submysterygift" in l:
                                last_subgift = datetime.fromtimestamp(int(l.split("\t")[1]))
                                if (datetime.fromtimestamp(int(ts)) - last_subgift).seconds < 15:
                                    check = False
                                    break
                        
                        if check:
                            push_element_to_queue(ts, username, user_id, event_type, sub_tier, sub_months, quantity)

                elif "PING" in line:
                    self.socket_irc.send(("PONG :tmi.twitch.tv\r\n").encode())

            readbuffer = ""

    def run(self):
        self.socket_irc = self.connect()
        readbuffer = self.socket_irc.recv(65536).decode()
        count = 0
        while "Login unsuccessful" in readbuffer and count <= 10:
            self.socket_irc.close()
            time.sleep(1)
            self.socket_irc = self.connect()
            readbuffer = self.socket_irc.recv(65536).decode()
            count += 1

        if count > 10 and "Login unsuccessful" in readbuffer:
            print(f"> Login unsuccessful {self.channel}")
            self.socket_irc.close()
            return

        self.socket_irc.settimeout(10)
        
        self.start_listen()


def main():
    last_exception = datetime.now()

    try:
        t = ListenChatThread(CHANNEL)
        t.start()
    except KeyboardInterrupt:
        pass
    except Exception as e:
        tb = traceback.format_exc()
        print(tb)
        tg_message = f"Error in subvision events tracker\n{tb}"
        # notify_error(tg_message)

        if datetime.now() - last_exception > datetime.timedelta(days=1):
            last_exception = datetime.now()
            os.system("systemctl restart subvision-subs-tracker")


if __name__ == "__main__":
    main()
