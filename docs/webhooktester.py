from flask import request, Flask
 
# from svix.webhooks import Webhook, WebhookVerificationError
app = Flask(__name__)
# secret = "whsec_MfKQ9r8GKYqrTwjUPD8ILPZIo2LaLaSw"
 
@app.route('/', methods=['POST'])
def webhook_handler():
    headers = request.headers
    payload = request.get_data()
 
    # try:
        # wh = Webhook(secret)
        # msg = wh.verify(payload, headers)
    print(headers)
    print(payload)
    # except WebhookVerificationError as e:
    #     return ('', 400)
 
    # Do something with the message...
 
    return ('', 204)


app.run(port=3000)