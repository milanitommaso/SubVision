import json

BOT_OWNER = 'milanitommasobot'
NICK = 'milanitommasobot'
SERVER = 'irc.twitch.tv'
CHANNEL = 'milanitommaso'
SUBS_EVENTS = ['sub', 'resub', 'subgift', 'submysterygift', 'giftpaidupgrade', 'rewardgift', 'anongiftpaidupgrade']
SAVE_RAW_LOGS_EVERY = 300 # 5 minutes

# configuration for the events_to_regions script
REGION_PER_100_BITS = 1
REGION_PER_PRIME = 1
REGION_PER_TIER1 = 2
REGION_PER_TIER2 = 3
REGION_PER_TIER3 = 4
REGION_PER_GIFTED_SUB = 2

# take the credentials from the json file
try:
    with open('credentials.json') as json_file:
        data = json.load(json_file)
        PASSWORD = data['password_irc']
        AUTHORIZATION_TWITCH_API = data['authorization_twitch_api']
        CLIENT_ID = data['client_id']
        SQS_QUEUE_NAME = data['sqs_queue_name']
except FileNotFoundError:
    print("credentials.json not found, please create it")
    exit(1)
