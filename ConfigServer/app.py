from flask import Flask,request,jsonify,render_template

app = Flask(__name__)
configs = {"MaxBucketSize":60,
            "EnableLogin":False}


@app.route('/', methods=['GET', 'POST'])
def Homepage():
    if request.method == 'POST':
        req = request.get_json() 
        configs[req["name"]] = req["value"]
        return jsonify({"operation":"success"})
    else: 
        return render_template('hello.html', config=configs)


@app.route('/<key>', methods=['GET'])
def getKey(key):
    if(key in configs):
        return str(configs[key])
    else:
        return jsonify({"error":True})