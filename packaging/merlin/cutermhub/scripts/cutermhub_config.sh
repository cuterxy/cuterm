#!/bin/sh
# cuterm-hub 插件服务控制脚本：软件中心页面提交、开机启动（init.d 软链）都会调用本脚本。
source /koolshare/scripts/base.sh
alias echo_date='echo 【$(TZ=UTC-8 date -R +%Y年%m月%d日\ %X)】:'
eval $(dbus export cutermhub_)

BIN=/koolshare/bin/cuterm-hub
# 配置与日志的持久化目录（jffs 分区，重启不丢失）；作为 cuterm-hub 的 HOME，
# 实际配置文件为 $HUB_HOME/.cuterm-hub/config.json，日志为同目录 cuterm-hub.log
HUB_HOME=/koolshare/configs/cutermhub
DEFAULT_PORT=7682

hub_port(){
	local port=$(sed -n 's/.*"port"[: ]*\([0-9][0-9]*\).*/\1/p' $HUB_HOME/.cuterm-hub/config.json 2>/dev/null | head -n1)
	echo ${port:-$DEFAULT_PORT}
}

start_hub(){
	if [ -n "$(pidof cuterm-hub)" ]; then
		echo_date "cuterm-hub 已在运行，跳过启动！"
	else
		mkdir -p $HUB_HOME
		# cuterm-hub 启动后会自动转入后台运行
		export HOME=$HUB_HOME
		$BIN >/dev/null 2>&1
		sleep 1
		if [ -n "$(pidof cuterm-hub)" ]; then
			dbus set cutermhub_port=$(hub_port)
			echo_date "cuterm-hub 启动成功，管理页面：http://$(nvram get lan_ipaddr):$(hub_port)"
		else
			echo_date "cuterm-hub 启动失败，请查看日志：$HUB_HOME/.cuterm-hub/cuterm-hub.log"
		fi
	fi

	# check startup file
	if [ ! -L "/koolshare/init.d/S98cutermhub.sh" ]; then
		ln -sf /koolshare/scripts/cutermhub_config.sh /koolshare/init.d/S98cutermhub.sh
	fi
}

stop_hub(){
	if [ -n "$(pidof cuterm-hub)" ]; then
		kill $(pidof cuterm-hub) >/dev/null 2>&1
		echo_date "cuterm-hub 已停止！"
	else
		echo_date "cuterm-hub 未在运行！"
	fi
}

ACTION=${1:-$ACTION}
case $ACTION in
start)
	# 开机 / WAN 拨号触发：只有开启状态下才启动
	if [ "$cutermhub_enable" == "1" ]; then
		logger "[软件中心]: 启动cuterm-hub！"
		start_hub
	else
		logger "[软件中心]: cuterm-hub未设置开机启动，跳过！"
	fi
	;;
stop)
	stop_hub
	;;
esac

case $2 in
1)
	# 软件中心页面提交
	http_response "$1"
	if [ "$cutermhub_enable" == "1" ]; then
		echo_date "开启 cuterm-hub！"
		start_hub
	else
		echo_date "关闭 cuterm-hub！"
		stop_hub
	fi
	;;
esac
