# rabbitmq_example
пример rabbitmq воркеров на го,  с реконнетом и переиспользованием коннекта между воркерами.

internal/rabbitmq/client.go internal/rabbitmq/channel.go. 
1. Отслеживаю каналы notifyConnClose и notifyChanClose и при обрыве связи горутины handleReConnect восстанавливают подключение
2. После реконнекта через канал notified паблишеры и консумеры оповещаются о произошедшем реконнете и переинициализируются


1. TODO добавить тесты 
