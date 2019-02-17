# Bitmark Node Documentation

## Introduction

This repo is fork from [bitmark-node](https://github.com/bitmark-inc/bitmark-node) and made some change for interview assignment.
The modified bitmark-node uploads the node information when bitmarkd is running.

## Installation

**To install the Bitmark node software, please complete the following 4 steps:**

### 1. Install Docker

The Bitmark node software is distributed as a standalone [Docker container](https://www.docker.com/what-container) which requires you to first install Docker for your operating system:


- [Get Docker for MacOS](https://store.docker.com/editions/community/docker-ce-desktop-mac)
- [Get Docker for Windows](https://store.docker.com/editions/community/docker-ce-desktop-windows)
- [Get Docker for CentOS](https://store.docker.com/editions/community/docker-ce-server-centos)
- [Get Docker for Debian](https://store.docker.com/editions/community/docker-ce-server-debian)
- [Get Docker for Fedora](https://store.docker.com/editions/community/docker-ce-server-fedora)
- [Get Docker for Ubuntu](https://store.docker.com/editions/community/docker-ce-server-ubuntu)
- [Get Docker for AWS](https://store.docker.com/editions/community/docker-ce-aws)
- [Get Docker for Azure](https://store.docker.com/editions/community/docker-ce-azure)

### 2. Download the Bitmark Node

After successfully installing Docker, you can download the Bitmark node software. To do so, first open a command-line terminal or shell application, such as Terminal on the Mac or `cmd.exe` on Windows. Then enter the following command to download the Bitmark node software:

```
docker pull seanchuang/bitmark-node-map
```


After entering the pull command, the download sequence should begin in the terminal. You will receive the following message after the download is completed successfully:

```
Status: Downloaded newer image for bitmark/bitmark-node:latest
```


### 3. Run Bitmark Node

After the Bitmark node software has successfully downloaded, copy and paste the following command into the command-line terminal to run the Bitmark node:

```
docker run -d --name bitmarkNode -p 9980:9980 \
-p 2136:2136 -p 2130:2130 \
-e PUBLIC_IP=[YOUR_PUBLIC_IP] \
-e SERVER_IP=[SERVER_IP] \
-v $HOME/bitmark-node-data/db:/.config/bitmark-node/db \
-v $HOME/bitmark-node-data/data:/.config/bitmark-node/bitmarkd/bitmark/data \
-v $HOME/bitmark-node-data/data-test:/.config/bitmark-node/bitmarkd/testing/data \
bitmark/bitmark-node
```

Please remember to replace `[YOUR_PUBLIC_IP]` and `[SERVER_IP]` to your node public ip and [nodemap_service](https://github.com/jseanchuang/nodemap-service) ip. The map_servie is a centralized service that records the node information. The bitmakrd will automatically uploads the information to nodemap_service when it is running.

Once the Bitmark node has successfully started, it will return a 64-character hexidecimal string that represents the Bitmark node's Docker container ID, such as:

```
dc78231837f2d320f24ed70c9f8c431abf52e7556bbdec257546f3acdbda5cd2
```


When the Bitmark node software is started up for the first time, it will generate a Bitmark account for you, including your public and private keypairs.

For an explanation of each of the above `run` command options, please enter the following command into the terminal:

```
docker run --help
 ```



### 4. Start Services in Web Interface

The Bitmark node includes a web-based user interface to monitor and control the Bitmark node within a web browser. After running the Bitmark node in step 3, you should launch the web UI to start the `bitmarkd` and optional `recorderd` programs.

On most computer systems, the web UI can be accessed on port `9980` of the `localhost` address (`127.0.0.1`) by clicking the following link:

> [http://127.0.0.1:9980](http://127.0.0.1:9980).

After loading web UI, you should use it to start the two main Bitmark node software programs:

1. `bitmarkd` — responsible for verifying Bitmark transactions and recording them in the Bitmark blockchain (required for all Bitmark nodes)
2. `recorderd` — required for solving the Bitmark blockchain's proof-of-work algorithm, which qualifies nodes to win blocks and receive monetary compensation (optional)

After starting the `bitmarkd` node for the first time, the node will go through an initial `Resynchronizing` phase in which a copy of the current Bitmark blockchain will be downloaded to your Bitmark node. Once the blockchain resynchronization has completed, your Bitmark node will begin verifying and recording transactions for the current block.
