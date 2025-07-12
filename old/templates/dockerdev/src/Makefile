VFLAGS=-v

ifdef V
  DEBUG_SUFFIX=--terragrunt-log-level debug --terragrunt-debug
else
  DEBUG_SUFFIX=
endif

up:
	cd infra && docker compose --profile all up -d

down:
	cd infra && docker compose --profile all down

build:
	cd infra && docker compose build jumpbox

restart: down up

rs: build up shell


check_app_name:
ifndef APP
	$(error APP is not set)
endif

build_app: check_app_name
	cd infra && docker compose build $(APP)

logs:
	cd infra && docker compose logs -f

shell:
	cd infra && docker compose exec -ti jumpbox zsh

shell-prod:
	cd infra && docker compose exec -ti prod zsh

PHONY: up down restart build logs shell


clean:
	find . -type d -name ".terraform" -prune -exec rm -rf {} \;
	find . -type d -name ".terragrunt-cache" -prune -exec rm -rf {} \;
	find . -type f -name "_backend.tf" -prune -exec rm -rf {} \;
	find . -type f -name "_provider.tf" -prune -exec rm -rf {} \;
	find . -type f -name ".terraform.lock.hcl" -prune -exec rm -rf {} \;

switch_terraform:
	cd $(APP) && tfswitch -b ~/.local/bin/terraform -u

init: check_app_name switch_terraform
	cd $(APP) && terragrunt run-all fmt $(DEBUG_SUFFIX)
	cd $(APP) && terragrunt run-all init $(DEBUG_SUFFIX)

plan: check_app_name switch_terraform
	cd $(APP) && terragrunt run-all plan $(DEBUG_SUFFIX)

apply: check_app_name init
	cd $(APP) && terragrunt run-all apply --terragrunt-non-interactive $(DEBUG_SUFFIX)

destroy: check_app_name switch_terraform
	cd $(APP) && terragrunt run-all destroy --terragrunt-non-interactive $(DEBUG_SUFFIX)

PHONY: up down restart build logs shell start pm2 generate_pm2 destroy
