# Đi tù online (v2)

Game 3D nhiều người chơi chạy trên trình duyệt: thế giới nhà tù liền mạch nhiều tầng, đi bộ/chạy/nhảy, góc nhìn thứ nhất và thứ ba, vào từng phòng, tương tác đồ vật, cai ngục tuần tra, vượt ngục theo nhóm. Dữ liệu tài khoản và tiến trình lưu bền vững trong PostgreSQL.

- **Client:** Three.js (đặt sẵn trong `web/static/vendor`, không cần CDN), JavaScript thuần, đồ họa low-poly dựng bằng hình khối, geometry tĩnh được gộp thành 2 mesh.
- **Server:** Go, WebSocket, server là nguồn sự thật (authoritative), tick 20 Hz, snapshot 10 Hz theo vùng quan sát.
- **Database:** PostgreSQL (`lib/pq`), migration nhúng sẵn trong binary. Không đặt `DATABASE_URL` thì chạy bằng bộ nhớ (chỉ để thử, mất dữ liệu khi khởi động lại).

## Chạy thử

```bash
make run                      # http://localhost:8080 (bộ nhớ, không lưu)
docker compose up --build     # có PostgreSQL, http://localhost:8080
make test                     # go test -race ./... (đặt TEST_DATABASE_URL để chạy cả test PostgreSQL)
make parity                   # kiểm tra vật lý JS khớp vật lý Go + smoke test client
```

Điều khiển: WASD đi, Shift chạy, Space nhảy, chuột xoay camera (bấm vào màn hình để khóa chuột), **V** đổi góc nhìn thứ nhất/thứ ba, **E** tương tác (cửa, giường, bàn làm việc, bàn học, xà đơn, quầy căng tin, hàng rào), **1-4** cử chỉ (vẫy tay, nhảy múa, cười, ngồi), **Tab** túi đồ, **O** trang phục và biểu cảm, **L** xếp hạng, **Enter** chat. Có điều khiển cảm ứng cho điện thoại.

Chat: tab Toàn trại, Phòng (người cùng phòng), Riêng. Chat riêng: `/w Tên nội dung` (tên có dấu cách: `/w "Bot an" nội dung`). Báo cáo vi phạm: `/report Tên lý do`.

## Cấu hình (biến môi trường)

| Biến | Mặc định | Ý nghĩa |
|---|---|---|
| `PORT` | 8080 | Cổng HTTP (Render tự đặt) |
| `DATABASE_URL` | trống | Chuỗi kết nối PostgreSQL. Local không TLS: thêm `?sslmode=disable` |
| `AUTH_SECRET` | ngẫu nhiên | Khóa ký token. **Bắt buộc đặt cố định khi triển khai**, nếu không mọi người bị đăng xuất khi khởi động lại |
| `ADMIN_KEY` | trống | Bật API quản trị (header `X-Admin-Key`) |
| `MAX_PLAYERS_PER_PRISON` | 24 | Số người tối đa mỗi trại (mỗi trại là một thế giới riêng) |
| `MAX_INSTANCES` | 200 | Số trại tối đa mỗi tiến trình |
| `GAME_DAY_SECONDS` | 600 | Một ngày trong game dài bao nhiêu giây |
| `TRUST_PROXY` | trống | `1` khi chạy sau proxy (Render) để lấy IP thật cho giới hạn request |
| `ALLOWED_ORIGINS` | cùng origin | Danh sách origin được phép mở WebSocket, cách nhau bằng dấu phẩy |
| `PBKDF2_ITER` | 150000 | Số vòng băm mật khẩu |
| `DISABLE_RATE_LIMITS` | trống | `1` chỉ để chạy load test, **không dùng khi chạy thật** |

## Triển khai lên Render

1. Đẩy mã lên GitHub, tạo **Blueprint** từ `render.yaml`: Render tạo một web service Docker và một PostgreSQL riêng, tự đặt `DATABASE_URL`, `AUTH_SECRET`, `ADMIN_KEY`.
2. Gói `free` của web service bị ngủ khi không có người truy cập và ngắt WebSocket, nên blueprint dùng `starter`. Database gói free của Render hết hạn sau một thời gian, hãy dùng gói trả phí cho dữ liệu thật. Tên gói trong `render.yaml` có thể thay đổi theo Render, hãy kiểm tra lại trong bảng điều khiển.
3. Kiểm tra `GET /healthz` trả về `{"ok":true,...}`.

Mở rộng: mỗi tiến trình chứa nhiều "trại"; người chơi cùng trại phải ở cùng một tiến trình. Hiện tại trạng thái trại nằm trong RAM của tiến trình nên mỗi dịch vụ chạy **một** instance. Muốn chạy nhiều máy cần thêm tầng định tuyến theo trại (chưa có).

## Quản trị

```bash
curl -X POST $URL/api/admin/ban  -H "X-Admin-Key: $ADMIN_KEY" -d '{"username":"troll","minutes":60,"reason":"spam"}'   # minutes=0 để gỡ khóa
curl -X POST $URL/api/admin/mute -H "X-Admin-Key: $ADMIN_KEY" -d '{"username":"troll","minutes":30}'
curl          $URL/api/admin/reports -H "X-Admin-Key: $ADMIN_KEY"
```

## Kiến trúc

```
cmd/server      điểm vào, cấu hình, tắt máy êm (lưu toàn bộ người chơi)
cmd/loadtest    giả lập N người chơi, đo độ trễ, thông lượng, RAM server
internal/world  bản đồ (Build), vật lý di chuyển/va chạm/cầu thang (StepBody)
internal/game   mô phỏng một trại: tick, snapshot, chat, hành động, cai ngục, vượt ngục, nhiệm vụ, giao dịch
internal/store  PostgreSQL + bản bộ nhớ, migration nhúng
internal/auth   PBKDF2-HMAC-SHA256, token ký HMAC
internal/api    HTTP, WebSocket, hub gán người chơi vào trại, quản trị
web/static      client (index.html, js/…)
tools/parity    kiểm tra JS và Go cho cùng kết quả vật lý
```

Giao thức WebSocket (JSON, khóa ngắn). Client gửi: `hello`, `in` (seq, mx, mz, yaw, j, r), `chat`, `use`, `door`, `eat`, `buy`, `craft`, `offer`, `offer_reply`, `emote`, `face`, `outfit`, `report`, `cancel`, `ping`. Server gửi: `welcome`, `snap` (ack, vị trí của bạn, người xung quanh, người mới, người rời đi), `me`, `door`, `act`, `clock`, `chat`, `toast`, `shop`, `offer`, `tp`, `err`.

Đồng bộ: client dự đoán chuyển động bằng đúng thuật toán của server (cùng bước 50 ms), server xác nhận bằng `ack`, client phát lại các input chưa được xác nhận. Người chơi khác được nội suy với độ trễ hiển thị 120 ms. Mất kết nối: nhân vật ở lại trong thế giới 60 giây, đăng nhập lại là vào đúng chỗ; quá 60 giây thì được lưu và gỡ khỏi trại.

## Chống gian lận và an toàn

Vị trí, tốc độ, nhảy, va chạm, tiền, vật phẩm, kết quả hành động đều do server tính. Mỗi kết nối bị giới hạn 60 tin nhắn mỗi giây, tin nhắn tối đa 4 KB, chat 1 tin mỗi 0,8 giây. Đăng ký và đăng nhập giới hạn theo IP. Mật khẩu băm PBKDF2, token hết hạn sau 7 ngày. Mọi nội dung người chơi nhập đều hiển thị bằng `textContent`. Khi một tài khoản vào từ nơi khác, kết nối cũ bị đóng.

## Chưa có (làm ở các giai đoạn sau)

Bạn bè, băng nhóm, chợ người chơi dùng chung giữa các trại, sự kiện cộng đồng, vật phẩm trang trí và cửa hàng trang phục, trang quản trị dạng giao diện web, chạy nhiều máy, âm thanh, hoạt ảnh nhân vật dạng xương (hiện là tư thế dựng bằng code).
