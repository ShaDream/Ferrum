echo "Start init data..."
./ferrum-admin --config=config_docker_w_redis.json --resource=realm --operation=create --value='{"name": "WissanceFerrumDemo", "token_expiration": 600, "refresh_expiration": 300}'
./ferrum-admin --config=config_docker_w_redis.json --resource=client --operation=create --params="WissanceFerrumDemo" --value='{"id": "d4dc483d-7d0d-4d2e-a0a0-2d34b55e6666", "name": "WissanceWebDemo", "type": "confidential", "auth": {"type": 1, "value": "fb6Z4RsOadVycQoeQiN57xpu8w8wTEST"}}'
./ferrum-admin --config=config_docker_w_redis.json --resource=user --operation=create --params="WissanceFerrumDemo" --value='{"info": {"sub": "667ff6a7-3f6b-449b-a217-6fc5d9ac6890", "email_verified": true, "roles": ["admin"], "name": "J.Doe", "preferred_username": "jodo", "given_name": "John", "family_name": "Doe"}, "credentials": {"password": "1s2d3f4g90xs"}}'
./ferrum-admin --config=config_docker_w_redis.json --resource=user --operation=create --params="WissanceFerrumDemo" --value='{"info": {"sub": "667ff6a7-3f6b-449b-a217-6fc5d9ac6891", "email_verified": true, "roles": ["admin"], "name": "J.Doe", "preferred_username": "jado", "given_name": "Jane", "family_name": "Doe"}, "credentials": {"password": "1s2d3f4g90xs"}}'
echo "End init data."