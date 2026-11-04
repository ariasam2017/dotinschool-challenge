# shellcheck shell=bash
# =============================================================================
# Certificate helpers shared by both management CLIs: scripts/install.sh (the
# systemd install) and the wrapper build/docker-install.sh writes (Docker).
# Sourced, never executed. Shipped in the release bundle under build/, so both
# installs find it at /opt/dotinschool-challenge/build/ssl-tools.sh.
#
# Covers the "bring your own certificate" path (Certum, a company CA, any
# commercial CA): make a key + CSR on the server, then import whatever the CA
# sends back (PEM, DER .cer/.crt, PKCS#7 .p7b, or a .pfx/.p12 holding the key),
# put the chain in the right order and refuse anything that would not work.
# Needs OpenSSL >= 1.1.1 (-checkhost, -addext, -ext).
# =============================================================================

SSL_RENEW_WARN_DAYS=30

# whole days until the certificate's notAfter (negative = expired)
ssl_days_left(){
  local end; end=$(openssl x509 -in "$1" -noout -enddate 2>/dev/null | cut -d= -f2-)
  [[ -n "$end" ]] || return 1
  echo $(( ( $(date -d "$end" +%s) - $(date +%s) ) / 86400 ))
}

ssl_show_info(){ # cert.pem
  if [[ ! -s "$1" ]]; then echo "  (no certificate installed)"; return 0; fi
  local days; days=$(ssl_days_left "$1" || echo "?")
  echo "  Names   : $(openssl x509 -in "$1" -noout -ext subjectAltName 2>/dev/null | tail -n +2 | sed 's/^ *//')"
  echo "  Issuer  : $(openssl x509 -in "$1" -noout -issuer 2>/dev/null | sed 's/^issuer= *//')"
  echo "  Expires : $(openssl x509 -in "$1" -noout -enddate 2>/dev/null | cut -d= -f2-)  (${days} days left)"
  if [[ "$days" =~ ^-?[0-9]+$ ]] && (( days < SSL_RENEW_WARN_DAYS )); then
    echo "  [!] Renew this certificate soon."
  fi
}

# Any certificate file -> the PEM certificates it contains, in $2.
# Understands PEM (one or many), DER (.cer/.crt) and PKCS#7 (.p7b/.p7c, PEM or DER).
ssl_to_pem_certs(){
  local src="$1" out="$2" text=""
  if grep -q -- "-----BEGIN PKCS7-----" "$src" 2>/dev/null; then
    text=$(openssl pkcs7 -in "$src" -print_certs 2>/dev/null) || true
  elif grep -q -- "-----BEGIN CERTIFICATE-----" "$src" 2>/dev/null; then
    text=$(cat "$src")
  else
    text=$(openssl x509 -inform DER -in "$src" 2>/dev/null) \
      || text=$(openssl pkcs7 -inform DER -in "$src" -print_certs 2>/dev/null) || true
  fi
  printf '%s\n' "$text" | sed -n '/-----BEGIN CERTIFICATE-----/,/-----END CERTIFICATE-----/p' > "$out"
  grep -q -- "-----BEGIN CERTIFICATE-----" "$out"
}

# certs.pem + key.pem -> out.pem with the certificate matching the key first,
# then the intermediates in the order given. Self-signed roots are dropped
# (clients already have them). Returns 2 when no certificate matches the key.
ssl_build_fullchain(){
  local certs="$1" key="$2" out="$3" d keypub leaf="" c subj iss
  d=$(mktemp -d)
  awk -v d="$d" '/-----BEGIN CERTIFICATE-----/{n++; f=sprintf("%s/c%03d.pem", d, n)} n{print > f} /-----END CERTIFICATE-----/{close(f)}' "$certs"
  keypub=$(openssl pkey -in "$key" -pubout 2>/dev/null) || { rm -rf "$d"; return 1; }
  for c in "$d"/c*.pem; do
    [[ -s "$c" ]] || continue
    if [[ "$(openssl x509 -in "$c" -noout -pubkey 2>/dev/null)" == "$keypub" ]]; then leaf="$c"; break; fi
  done
  [[ -n "$leaf" ]] || { rm -rf "$d"; return 2; }
  cat "$leaf" > "$out"
  for c in "$d"/c*.pem; do
    [[ "$c" == "$leaf" ]] && continue
    subj=$(openssl x509 -in "$c" -noout -subject 2>/dev/null | sed 's/^subject= *//')
    iss=$(openssl x509 -in "$c" -noout -issuer 2>/dev/null | sed 's/^issuer= *//')
    [[ "$subj" == "$iss" ]] && continue
    cat "$c" >> "$out"
  done
  rm -rf "$d"
}

# Refuse a certificate that cannot work; warn about one that may cause trouble.
ssl_validate(){ # fullchain key domain
  local fc="$1" key="$2" domain="$3" days
  openssl x509 -in "$fc" -noout 2>/dev/null || { echo "[x] The certificate can't be read."; return 1; }
  openssl pkey -in "$key" -noout 2>/dev/null || { echo "[x] The private key can't be read."; return 1; }
  [[ "$(openssl x509 -in "$fc" -noout -pubkey 2>/dev/null)" == "$(openssl pkey -in "$key" -pubout 2>/dev/null)" ]] \
    || { echo "[x] This certificate does not belong to this private key."; return 1; }
  openssl x509 -in "$fc" -noout -checkend 0 >/dev/null 2>&1 || { echo "[x] The certificate has already expired."; return 1; }
  openssl x509 -in "$fc" -noout -checkhost "$domain" 2>/dev/null | grep -q "does match" \
    || { echo "[x] The certificate is not issued for ${domain}."; return 1; }
  if [[ $(grep -c -- "-----BEGIN CERTIFICATE-----" "$fc") -lt 2 ]]; then
    echo "[!] No intermediate certificate (CA bundle) included - some browsers and phones will show an"
    echo "    error. Add the bundle file your CA sent (Certum: the 'intermediate' / 'CA bundle' file)."
  fi
  if ! openssl verify -untrusted "$fc" "$fc" >/dev/null 2>&1; then
    echo "[!] The chain does not verify against this server's trusted CAs. That is normal for a private"
    echo "    company CA - make sure every user's device trusts that CA."
  fi
  days=$(ssl_days_left "$fc" || echo 0)
  (( days >= SSL_RENEW_WARN_DAYS )) || echo "[!] Expires in ${days} days - renew it soon."
  return 0
}

# Private key + certificate signing request for a CA you order from.
ssl_make_csr(){ # domain "extra names" outdir [organisation] [country]
  local domain="$1" extra="$2" out="$3" org="${4:-}" country="${5:-}" san n subj=""
  san="DNS:${domain}"
  for n in $extra; do san+=",DNS:${n}"; done
  [[ -n "$country" ]] && subj+="/C=${country}"
  [[ -n "$org" ]] && subj+="/O=${org//\//-}"
  subj+="/CN=${domain}"
  mkdir -p "$out"; chmod 700 "$out"
  ( umask 077
    openssl req -new -newkey rsa:2048 -nodes -keyout "$out/privkey.pem" -out "$out/request.csr" \
      -subj "$subj" -addext "subjectAltName=${san}" >/dev/null 2>&1 ) || return 1
  chmod 600 "$out/privkey.pem"
}

# Interactive import of a certificate the user already has. Writes a verified
# fullchain.pem + privkey.pem into stagedir. pendingdir is where ssl_make_csr
# left a key for this domain (if any).
ssl_import_interactive(){ # domain pendingdir stagedir
  local domain="$1" pending="$2" stage="$3" src bundle key pw rc
  echo "Upload the files your CA sent to this server first (e.g. with WinSCP, or:"
  echo "  scp certificate.crt root@<server>:/root/ ), then give their paths here."
  read -rp "Certificate file (.crt/.cer/.pem/.p7b, or a .pfx/.p12 that includes the key): " src
  src="${src/#\~/$HOME}"
  [[ -s "$src" ]] || { echo "[x] File not found: $src"; return 1; }
  : > "$stage/certs.pem"

  case "${src,,}" in
    *.pfx|*.p12)
      read -rsp "Password of the .pfx/.p12 file (Enter if none): " pw; echo
      export SSL_TOOLS_PW="$pw"
      if ! openssl pkcs12 -in "$src" -nocerts -nodes -passin env:SSL_TOOLS_PW 2>/dev/null | openssl pkey -out "$stage/privkey.pem" 2>/dev/null; then
        openssl pkcs12 -legacy -in "$src" -nocerts -nodes -passin env:SSL_TOOLS_PW 2>/dev/null | openssl pkey -out "$stage/privkey.pem" 2>/dev/null \
          || { unset SSL_TOOLS_PW; echo "[x] Could not open the .pfx/.p12 file (wrong password?)."; return 1; }
        openssl pkcs12 -legacy -in "$src" -nokeys -passin env:SSL_TOOLS_PW 2>/dev/null > "$stage/pfx.txt" || true
      else
        openssl pkcs12 -in "$src" -nokeys -passin env:SSL_TOOLS_PW 2>/dev/null > "$stage/pfx.txt" || true
      fi
      unset SSL_TOOLS_PW
      ssl_to_pem_certs "$stage/pfx.txt" "$stage/certs.pem" || { echo "[x] No certificate inside the .pfx/.p12 file."; return 1; }
      rm -f "$stage/pfx.txt"
      ;;
    *)
      ssl_to_pem_certs "$src" "$stage/certs.pem" || { echo "[x] No certificate found in $src."; return 1; }
      if [[ -s "$pending/privkey.pem" ]]; then
        echo "Using the private key created with this server's CSR for ${domain}."
        key="$pending/privkey.pem"
      else
        read -rp "Private key file (.key/.pem): " key
        key="${key/#\~/$HOME}"
        [[ -s "$key" ]] || { echo "[x] File not found: $key"; return 1; }
      fi
      if grep -q "ENCRYPTED" "$key" 2>/dev/null; then
        read -rsp "The private key is password-protected - password: " pw; echo
        SSL_TOOLS_PW="$pw" openssl pkey -in "$key" -passin env:SSL_TOOLS_PW -out "$stage/privkey.pem" 2>/dev/null \
          || { echo "[x] Wrong password for the private key."; return 1; }
      else
        openssl pkey -in "$key" -out "$stage/privkey.pem" 2>/dev/null || { echo "[x] $key is not a private key."; return 1; }
      fi
      ;;
  esac
  chmod 600 "$stage/privkey.pem"

  read -rp "CA bundle / intermediate certificate file (Enter if already included or none): " bundle
  bundle="${bundle/#\~/$HOME}"
  if [[ -n "$bundle" ]]; then
    [[ -s "$bundle" ]] || { echo "[x] File not found: $bundle"; return 1; }
    ssl_to_pem_certs "$bundle" "$stage/bundle.pem" || { echo "[x] No certificate found in $bundle."; return 1; }
    cat "$stage/bundle.pem" >> "$stage/certs.pem"; rm -f "$stage/bundle.pem"
  fi

  ssl_build_fullchain "$stage/certs.pem" "$stage/privkey.pem" "$stage/fullchain.pem"; rc=$?
  rm -f "$stage/certs.pem"
  if [[ $rc -eq 2 ]]; then echo "[x] None of these certificates belongs to this private key."; return 1; fi
  [[ $rc -eq 0 ]] || { echo "[x] Could not assemble the certificate chain."; return 1; }
  ssl_validate "$stage/fullchain.pem" "$stage/privkey.pem" "$domain" || return 1
  echo "[ok] Certificate checked:"
  ssl_show_info "$stage/fullchain.pem"
}

# "any", or a comma list of IPv4/IPv6 addresses / CIDRs
ssl_valid_cidrs(){
  local v="$1" p parts
  [[ "$v" == "any" ]] && return 0
  [[ -n "$v" ]] || return 1
  IFS=',' read -ra parts <<< "$v"
  for p in "${parts[@]}"; do
    p="${p// /}"
    [[ "$p" =~ ^[0-9]{1,3}(\.[0-9]{1,3}){3}(/[0-9]{1,2})?$ || "$p" =~ ^[0-9a-fA-F:]*:[0-9a-fA-F:]*(/[0-9]{1,3})?$ ]] || return 1
  done
}

# Ask which network may open the admin panel once the HTTPS lock is on.
# Prompts on stderr; the chosen value on stdout.
ssl_prompt_admin_cidrs(){ # current-value
  local cur="$1" sug="" v
  [[ -n "${SSH_CLIENT:-}" ]] && sug="${SSH_CLIENT%% *}"
  {
    echo
    echo "Which network may open the ADMIN panel once the lock is on? Everyone else gets"
    echo "\"not found\" (the user panel stays open to all). Comma-separated IPs or CIDRs,"
    echo "e.g. 10.10.0.0/16,192.168.5.7. Type: any - only if Cloudflare Access or a VPN"
    echo "already restricts the admin panel."
    if [[ -n "$sug" ]]; then echo "You are connected from ${sug} right now."; fi
  } >&2
  read -rp "Allowed IPs/CIDRs [${cur:-$sug}]: " v
  v="${v:-${cur:-$sug}}"; v="${v// /}"
  ssl_valid_cidrs "$v" || return 1
  echo "$v"
}
