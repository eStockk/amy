#!/bin/sh
set -eu

HOST_ROOT="${HOST_ROOT:-/host}"
TEXTFILE_DIR="${TEXTFILE_DIR:-/textfile}"
INTERVAL="${HOST_METRICS_INTERVAL:-15}"
TOP_LIMIT="${HOST_METRICS_TOP_LIMIT:-15}"
MINECRAFT_DIR="${MINECRAFT_DIR:-/opt/amy/minecraft}"
APP_DIR="${APP_DIR:-/opt/amy/app}"
SUPPORT_DIR="${SUPPORT_DIR:-/var/lib/docker/volumes/app_support-data/_data}"

mkdir -p "$TEXTFILE_DIR"

label_escape() {
  printf '%s' "$1" | sed 's/\\/\\\\/g; s/"/\\"/g'
}

metric_name_escape() {
  printf '%s' "$1" | tr '[:upper:] ' '[:lower:]_' | tr -cd 'a-z0-9_:'
}

read_meminfo_bytes() {
  key="$1"
  awk -v key="$key" '$1 == key ":" {printf "%.0f", $2 * 1024}' "$HOST_ROOT/proc/meminfo"
}

to_bytes() {
  value="$(printf '%s' "$1" | sed 's/,/./g; s/^[[:space:]]*//; s/[[:space:]]*$//')"
  number="$(printf '%s' "$value" | sed -E 's/^([0-9.]+).*/\1/')"
  unit="$(printf '%s' "$value" | sed -E 's/^[0-9.]+[[:space:]]*//; s/B$//')"
  [ -n "$number" ] || number=0
  awk -v n="$number" -v u="$unit" 'BEGIN {
    if (u == "" || u == "B") m = 1;
    else if (u == "k" || u == "K" || u == "KB") m = 1000;
    else if (u == "Ki" || u == "KiB") m = 1024;
    else if (u == "M" || u == "MB") m = 1000 * 1000;
    else if (u == "Mi" || u == "MiB") m = 1024 * 1024;
    else if (u == "G" || u == "GB") m = 1000 * 1000 * 1000;
    else if (u == "Gi" || u == "GiB") m = 1024 * 1024 * 1024;
    else if (u == "T" || u == "TB") m = 1000 * 1000 * 1000 * 1000;
    else if (u == "Ti" || u == "TiB") m = 1024 * 1024 * 1024 * 1024;
    else m = 1;
    printf "%.0f", n * m;
  }'
}

du_bytes() {
  path="$1"
  if [ -e "$path" ]; then
    du -sk "$path" 2>/dev/null | awk '{print $1 * 1024}'
  else
    printf '0'
  fi
}

read_cpu_sample() {
  awk '/^cpu / {
    total = 0;
    for (i = 2; i <= NF; i++) total += $i;
    idle = $5 + $6;
    print total, idle;
  }' "$HOST_ROOT/proc/stat"
}

prev_cpu_total=0
prev_cpu_idle=0
prev_process_file=/tmp/amy_process_cpu.prev
touch "$prev_process_file"

while true; do
  out="$TEXTFILE_DIR/amy_host.prom"
  tmp="$out.tmp"
  now="$(date +%s)"
  set -- $(read_cpu_sample)
  cpu_total="$1"
  cpu_idle="$2"
  total_delta=$((cpu_total - prev_cpu_total))
  idle_delta=$((cpu_idle - prev_cpu_idle))
  if [ "$prev_cpu_total" -eq 0 ] || [ "$total_delta" -le 0 ]; then
    host_cpu_percent=0
  else
    host_cpu_percent="$(awk -v t="$total_delta" -v i="$idle_delta" 'BEGIN { printf "%.4f", 100 * (t - i) / t }')"
  fi

  mem_total="$(read_meminfo_bytes MemTotal)"
  mem_available="$(read_meminfo_bytes MemAvailable)"
  mem_used=$((mem_total - mem_available))

  current_process_file=/tmp/amy_process_cpu.current
  process_rows=/tmp/amy_process_rows.current
  : > "$current_process_file"
  : > "$process_rows"

  for proc_dir in "$HOST_ROOT"/proc/[0-9]*; do
    [ -r "$proc_dir/stat" ] || continue
    pid="${proc_dir##*/}"
    stat_line="$(cat "$proc_dir/stat" 2>/dev/null || true)"
    [ -n "$stat_line" ] || continue
    after="${stat_line#*) }"
    set -- $after
    utime="${12:-0}"
    stime="${13:-0}"
    proc_ticks=$((utime + stime))
    printf '%s %s\n' "$pid" "$proc_ticks" >> "$current_process_file"

    prev_ticks="$(awk -v pid="$pid" '$1 == pid {print $2; exit}' "$prev_process_file" 2>/dev/null || true)"
    [ -n "$prev_ticks" ] || prev_ticks="$proc_ticks"
    proc_delta=$((proc_ticks - prev_ticks))
    [ "$proc_delta" -ge 0 ] || proc_delta=0
    if [ "$total_delta" -gt 0 ]; then
      proc_cpu="$(awk -v p="$proc_delta" -v t="$total_delta" 'BEGIN { printf "%.4f", 100 * p / t }')"
    else
      proc_cpu=0
    fi
    rss="$(awk '/^VmRSS:/ {printf "%.0f", $2 * 1024}' "$proc_dir/status" 2>/dev/null || printf '0')"
    name="$(cat "$proc_dir/comm" 2>/dev/null | tr '\n' ' ' | cut -c1-48)"
    cmd="$(tr '\000' ' ' < "$proc_dir/cmdline" 2>/dev/null | sed 's/[[:space:]]*$//' | cut -c1-140)"
    [ -n "$cmd" ] || cmd="$name"
    printf '%s %s %s|%s|%s|%s\n' "$proc_cpu" "$rss" "$pid" "$name" "$cmd" "$pid" >> "$process_rows"
  done

  {
    echo '# HELP amy_host_cpu_usage_percent Host CPU usage over the last collection interval.'
    echo '# TYPE amy_host_cpu_usage_percent gauge'
    printf 'amy_host_cpu_usage_percent %.4f\n' "$host_cpu_percent"
    echo '# HELP amy_host_memory_total_bytes Host total memory.'
    echo '# TYPE amy_host_memory_total_bytes gauge'
    printf 'amy_host_memory_total_bytes %s\n' "$mem_total"
    echo '# HELP amy_host_memory_available_bytes Host available memory.'
    echo '# TYPE amy_host_memory_available_bytes gauge'
    printf 'amy_host_memory_available_bytes %s\n' "$mem_available"
    echo '# HELP amy_host_memory_used_bytes Host used memory.'
    echo '# TYPE amy_host_memory_used_bytes gauge'
    printf 'amy_host_memory_used_bytes %s\n' "$mem_used"

    echo '# HELP amy_process_cpu_percent Top host processes by CPU usage as percent of the whole machine.'
    echo '# TYPE amy_process_cpu_percent gauge'
    sort -nr -k1,1 "$process_rows" | head -n "$TOP_LIMIT" | while IFS='|' read -r head name cmd pid; do
      cpu="$(printf '%s' "$head" | awk '{print $1}')"
      printf 'amy_process_cpu_percent{pid="%s",name="%s",cmdline="%s"} %s\n' \
        "$(label_escape "$pid")" "$(label_escape "$name")" "$(label_escape "$cmd")" "$cpu"
    done

    echo '# HELP amy_process_memory_rss_bytes Top host processes by resident memory.'
    echo '# TYPE amy_process_memory_rss_bytes gauge'
    sort -nr -k2,2 "$process_rows" | head -n "$TOP_LIMIT" | while IFS='|' read -r head name cmd pid; do
      rss="$(printf '%s' "$head" | awk '{print $2}')"
      printf 'amy_process_memory_rss_bytes{pid="%s",name="%s",cmdline="%s"} %s\n' \
        "$(label_escape "$pid")" "$(label_escape "$name")" "$(label_escape "$cmd")" "$rss"
    done

    echo '# HELP amy_docker_container_size_bytes Docker container storage split by rootfs, writable layer and logs.'
    echo '# TYPE amy_docker_container_size_bytes gauge'
    if command -v docker >/dev/null 2>&1; then
      docker ps -aq 2>/dev/null | while read -r id; do
        [ -n "$id" ] || continue
        inspect="$(docker inspect --size --format '{{.Id}}|{{.Name}}|{{.Config.Image}}|{{.State.Status}}|{{.SizeRw}}|{{.SizeRootFs}}' "$id" 2>/dev/null || true)"
        [ -n "$inspect" ] || continue
        IFS='|' read -r full_id name image status size_rw size_rootfs <<EOF_DOCKER
$inspect
EOF_DOCKER
        short_id="$(printf '%s' "$full_id" | cut -c1-12)"
        name="${name#/}"
        log_size="$(du_bytes "$HOST_ROOT/var/lib/docker/containers/$full_id/$full_id-json.log")"
        for type_value in "writable:$size_rw" "rootfs:$size_rootfs" "logs:$log_size"; do
          storage_type="${type_value%%:*}"
          bytes="${type_value#*:}"
          [ -n "$bytes" ] || bytes=0
          printf 'amy_docker_container_size_bytes{id="%s",name="%s",image="%s",status="%s",type="%s"} %s\n' \
            "$(label_escape "$short_id")" "$(label_escape "$name")" "$(label_escape "$image")" "$(label_escape "$status")" "$(label_escape "$storage_type")" "$bytes"
        done
      done

      echo '# HELP amy_docker_container_state Container state as a labelled gauge. Running containers are 1, others are 0.'
      echo '# TYPE amy_docker_container_state gauge'
      docker ps -a --format '{{.ID}}|{{.Names}}|{{.Image}}|{{.Status}}' 2>/dev/null | while IFS='|' read -r id name image status; do
        [ -n "$id" ] || continue
        running=0
        case "$status" in
          Up*) running=1 ;;
        esac
        printf 'amy_docker_container_state{id="%s",name="%s",image="%s",status="%s"} %s\n' \
          "$(label_escape "$id")" "$(label_escape "$name")" "$(label_escape "$image")" "$(label_escape "$status")" "$running"
      done

      echo '# HELP amy_docker_container_cpu_percent Docker container CPU usage from docker stats.'
      echo '# TYPE amy_docker_container_cpu_percent gauge'
      echo '# HELP amy_docker_container_memory_usage_bytes Docker container memory usage from docker stats.'
      echo '# TYPE amy_docker_container_memory_usage_bytes gauge'
      echo '# HELP amy_docker_container_network_bytes_total Docker container network IO from docker stats.'
      echo '# TYPE amy_docker_container_network_bytes_total counter'
      docker stats --no-stream --format '{{.ID}}|{{.Name}}|{{.CPUPerc}}|{{.MemUsage}}|{{.NetIO}}' 2>/dev/null | while IFS='|' read -r id name cpu mem net; do
        cpu_value="$(printf '%s' "$cpu" | tr -d '%' | sed 's/,/./g')"
        mem_used_text="${mem%% / *}"
        net_rx_text="${net%% / *}"
        net_tx_text="${net##* / }"
        printf 'amy_docker_container_cpu_percent{id="%s",name="%s"} %s\n' "$(label_escape "$id")" "$(label_escape "$name")" "${cpu_value:-0}"
        printf 'amy_docker_container_memory_usage_bytes{id="%s",name="%s"} %s\n' "$(label_escape "$id")" "$(label_escape "$name")" "$(to_bytes "$mem_used_text")"
        printf 'amy_docker_container_network_bytes_total{id="%s",name="%s",direction="rx"} %s\n' "$(label_escape "$id")" "$(label_escape "$name")" "$(to_bytes "$net_rx_text")"
        printf 'amy_docker_container_network_bytes_total{id="%s",name="%s",direction="tx"} %s\n' "$(label_escape "$id")" "$(label_escape "$name")" "$(to_bytes "$net_tx_text")"
      done

      echo '# HELP amy_docker_volume_size_bytes Docker volume disk usage.'
      echo '# TYPE amy_docker_volume_size_bytes gauge'
      docker volume ls -q 2>/dev/null | while read -r volume; do
        [ -n "$volume" ] || continue
        mountpoint="$(docker volume inspect --format '{{.Mountpoint}}' "$volume" 2>/dev/null || true)"
        [ -n "$mountpoint" ] || continue
        printf 'amy_docker_volume_size_bytes{name="%s",mountpoint="%s"} %s\n' \
          "$(label_escape "$volume")" "$(label_escape "$mountpoint")" "$(du_bytes "$HOST_ROOT$mountpoint")"
      done

      echo '# HELP amy_docker_system_size_bytes Docker system disk usage by object type.'
      echo '# TYPE amy_docker_system_size_bytes gauge'
      docker system df --format '{{.Type}}|{{.TotalCount}}|{{.Active}}|{{.Size}}|{{.Reclaimable}}' 2>/dev/null | while IFS='|' read -r type total active size reclaimable; do
        [ -n "$type" ] || continue
        printf 'amy_docker_system_size_bytes{type="%s",state="total",total_count="%s",active_count="%s"} %s\n' \
          "$(label_escape "$type")" "$(label_escape "$total")" "$(label_escape "$active")" "$(to_bytes "$size")"
        reclaimable_size="${reclaimable%% *}"
        printf 'amy_docker_system_size_bytes{type="%s",state="reclaimable",total_count="%s",active_count="%s"} %s\n' \
          "$(label_escape "$type")" "$(label_escape "$total")" "$(label_escape "$active")" "$(to_bytes "$reclaimable_size")"
      done
    fi

    echo '# HELP amy_minecraft_storage_bytes Minecraft server storage usage by directory.'
    echo '# TYPE amy_minecraft_storage_bytes gauge'
    minecraft_host_path="$HOST_ROOT$MINECRAFT_DIR"
    printf 'amy_minecraft_storage_bytes{section="total",path="%s"} %s\n' "$(label_escape "$MINECRAFT_DIR")" "$(du_bytes "$minecraft_host_path")"
    if [ -d "$minecraft_host_path" ]; then
      for child in "$minecraft_host_path"/*; do
        [ -e "$child" ] || continue
        section="${child##*/}"
        display_path="$MINECRAFT_DIR/$section"
        printf 'amy_minecraft_storage_bytes{section="%s",path="%s"} %s\n' \
          "$(label_escape "$section")" "$(label_escape "$display_path")" "$(du_bytes "$child")"
      done
    fi

    echo '# HELP amy_storage_category_bytes Storage usage for Amy application areas.'
    echo '# TYPE amy_storage_category_bytes gauge'
    for item in \
      "docker|Docker root|/var/lib/docker" \
      "docker|Docker logs|/var/lib/docker/containers" \
      "app|Application|$APP_DIR" \
      "minecraft|Minecraft server|$MINECRAFT_DIR" \
      "support|Support archive|$SUPPORT_DIR" \
      "logs|System logs|/var/log"; do
      category="${item%%|*}"
      rest="${item#*|}"
      section="${rest%%|*}"
      path="${rest#*|}"
      printf 'amy_storage_category_bytes{category="%s",section="%s",path="%s"} %s\n' \
        "$(label_escape "$category")" "$(label_escape "$section")" "$(label_escape "$path")" "$(du_bytes "$HOST_ROOT$path")"
    done

    printf 'amy_host_metrics_last_success_timestamp_seconds %s\n' "$now"
  } > "$tmp"

  mv "$tmp" "$out"
  cp "$current_process_file" "$prev_process_file"
  prev_cpu_total="$cpu_total"
  prev_cpu_idle="$cpu_idle"
  sleep "$INTERVAL"
done
