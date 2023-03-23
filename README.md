# Real-time ChatGPT service, support GPT3/GPT4, support conversation and generate pictures from sentences

- [English README](README.md)
- [中文 README](README_CN.md)

## About chatgpt-service and chatgpt-stream

- chatgpt-service: [https://github.com/cookeem/chatgpt-service](https://github.com/cookeem/chatgpt-service) 
  - chatgpt-service is a backend service, used to receive chatGPT messages in real time, and feed back to chatGPT-stream in real time through websocket
- chatgpt-stream: [https://github.com/cookeem/chatgpt-stream](https://github.com/cookeem/chatgpt-stream) 
  - chatgpt-stream is a front-end service that receives messages returned by chatGPT-service in real time through websocket

## gitee

- [https://gitee.com/cookeem/chatgpt-service](https://gitee.com/cookeem/chatgpt-service) 
- [https://gitee.com/cookeem/chatgpt-stream](https://gitee.com/cookeem/chatgpt-stream) 

## Demo

- Real-time conversation mode

![](chatgpt-service.gif)

- Generate picture patterns from sentences

![](chatgpt-image.jpeg)

## Quick start

```bash
# Pull source code
git clone https://github.com/cookeem/chatgpt-service.git
cd chatgpt-service

# ChatGPT's registration page: https://beta.openai.com/signup
# ChatGPT registration tutorial: https://www.cnblogs.com/damugua/p/16969508.html
# ChatGPT API key management page: https://beta.openai.com/account/api-keys

# Modify the config.yaml configuration file, modify the apiKey, and change it to your openai.com API key
vi config.yaml
#  your openai.com API key
apiKey: "xxxxxx"

# create pictures directory
mkdir -p assets
chown -R 1000:1000 assets

# Start the service with docker-compose
docker-compose up -d

# Check service status
docker-compose ps   
     Name                    Command               State                  Ports                
-----------------------------------------------------------------------------------------------
chatgpt-service   /chatgpt-service/chatgpt-s ...   Up      0.0.0.0:59142->9000/tcp             
chatgpt-stream    /docker-entrypoint.sh ngin ...   Up      0.0.0.0:3000->80/tcp,:::3000->80/tcp


# To access the page, please ensure that your server can access the chatGPT API
# http://localhost:3000
```

- Enter the question directly, it will call the ChatGPT interface to return the answer
- Enter the picture description after `/image`, it will call the DALL-E2 interface to automatically generate pictures through the picture description

## How to build

```bash
# Pull build dependencies
go mod tidy
# Compile the project
go build

# Run the service
./chatgpt-service

# API url
# ws://localhost:9000/api/ws/chat

# Install wscat
npm install -g wscat

# Use wscat to test websocket, then enter the question you want to query
wscat --connect ws://localhost:9000/api/ws/chat

```