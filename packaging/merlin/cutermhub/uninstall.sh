#!/bin/sh

# stop
sh /koolshare/scripts/cutermhub_config.sh stop >/dev/null 2>&1

# remove files
rm -rf /koolshare/bin/cuterm-hub >/dev/null 2>&1
rm -rf /koolshare/res/icon-cutermhub.png >/dev/null 2>&1
rm -rf /koolshare/scripts/cutermhub* >/dev/null 2>&1
rm -rf /koolshare/webs/Module_cutermhub.asp >/dev/null 2>&1
rm -rf /koolshare/init.d/*cutermhub*.sh >/dev/null 2>&1

# remove myself
rm -rf /koolshare/scripts/uninstall_cutermhub.sh >/dev/null 2>&1
