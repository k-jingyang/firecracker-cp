# Doesn't seem necessary with SDK
MMDS_IPV4_ADDR=169.254.169.254
MMDS_NET_IF=eth0
ip route add ${MMDS_IPV4_ADDR} dev ${MMDS_NET_IF}