GOBUILD=go build
GOCLEAN=go clean
GOTEST=gotestsum
GOGET=$(GOCMD) get
BINARY_NAME=lirc.changer
GO_DIR=$(PWD)/cmd
BIN_DIR=$(PWD)/bin
REMOTE_URL=192.168.1.69
REMOTE_DIR=/storage/lirc.changer
    
$(BIN_DIR):
	mkdir $(BIN_DIR)
    
test: 
	$(GOTEST)

clean:
	# $(GOCLEAN)
	rm -r -f $(BIN_DIR)

build: | $(BIN_DIR)
	(cd $(GO_DIR) && CGO_ENABLED=0 GOOS=linux GOARCH=arm GOARM=5 $(GOBUILD) -o $(BIN_DIR)/$(BINARY_NAME) -v)

copy: $(BIN_DIR)/$(BINARY_NAME)
	ssh root@$(REMOTE_URL) "mkdir -p $(REMOTE_DIR)"
	scp $(BIN_DIR)/$(BINARY_NAME) root@$(REMOTE_URL):$(REMOTE_DIR)

date:
	ssh pi@$(RPI_URL) "uptime && date"	

copy-service:
	ssh root@$(REMOTE_URL) "mkdir -p $(REMOTE_DIR)/logs"
	scp config/lirc-changer.service root@$(REMOTE_URL):/storage/.config/system.d
	ssh root@$(REMOTE_URL) "systemctl enable lirc-changer.service"

restart:
	ssh root@$(REMOTE_URL) "systemctl restart lirc-changer.service"

stop:
	ssh root@$(REMOTE_URL) "systemctl stop lirc-changer.service"

logs:
	ssh root@$(REMOTE_URL) "cat $(REMOTE_DIR)/logs/service.err"		

ssh:
	ssh root@$(REMOTE_URL) 