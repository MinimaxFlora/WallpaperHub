export const SESSION_COOKIE = "wh_session";

export function sessionId(request: Request): string {
  const cookie = request.headers.get("Cookie") ?? "";
  const match = cookie.match(/(?:^|;\s*)wh_session=([^;]+)/);
  if (match) return match[1];
  const ip = request.headers.get("CF-Connecting-IP");
  return ip ? `ip:${ip}` : "anonymous";
}
