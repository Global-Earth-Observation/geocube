include ./env.mk

build-base:
	docker build -t $(REPOSITORY)/$(BASE_IMAGE):$(TAG) -f docker/Dockerfile.base-image .

create-base-repo:
	aws ecr create-repository --repository-name $(BASE_IMAGE)

push-base:
	 docker push $(REPOSITORY)/$(BASE_IMAGE):$(TAG)

build-server:
	docker build --build-arg "REPOSITORY=$(REPOSITORY)" -t $(REPOSITORY)/$(SERVER_IMAGE):$(TAG) -f docker/Dockerfile.server .

create-server-repo:
	aws ecr create-repository --repository-name $(SERVER_IMAGE)

push-server:
	docker push $(REPOSITORY)/$(SERVER_IMAGE):$(TAG)

create-init-db-repo:
	aws ecr create-repository --repository-name $(INIT_DB_IMAGE)

build-db-init:
	docker build -t $(REPOSITORY)/$(INIT_DB_IMAGE):$(TAG) -f docker/Dockerfile.init-db .

push-init-db:
	docker push $(REPOSITORY)/$(INIT_DB_IMAGE):$(TAG)
