# Đi tù online

Game multiplayer trình duyệt phong cách low-poly, backend Go authoritative, trạng thái trong RAM. Không có database, không ghi trạng thái người chơi xuống đĩa. Token riêng theo từng tab được lưu trong localStorage để hai tab trên cùng trình duyệt có thể tạo hai nhân vật độc lập.

## Chạy nhanh

```sh
go mod download
```

Yêu cầu Go 1.22+ và kết nối mạng ở lần đầu để tải `github.com/coder/websocket` (`make run` tự chạy `go mod download`). Three.js được ghim phiên bản `0.160.1` và tải từ jsDelivr (môi trường build hiện tại không truy cập được CDN để vendoring file local).

```sh
make run
# mở http://localhost:8080 trên hai tab
```

Build binary đơn:

```sh
make build
./bin/ditu
```

Test và kiểm tra tĩnh:

```sh
make test
make vet
go test -race ./...
```

Docker:

```sh
docker build -t ditu-online .
docker run --rm -p 8080:8080 -e PORT=8080 ditu-online
```

## Cấu hình

| Biến | Mặc định | Ý nghĩa |
|---|---:|---|
| `PORT` | `8080` | Cổng HTTP/WebSocket |
| `MAX_PLAYERS_PER_PRISON` | `60` | Số người tối đa mỗi trại |
| `GAME_DAY_SECONDS` | `90` | Thời lượng một ngày game |
| `SENTENCE_DAYS` | `30` | Cấu hình án mặc định dự kiến |

Mỗi người nhập trại nhận án ngẫu nhiên 20–40 ngày game. Phiên game mặc định dùng ngày game 90 giây, nên thời lượng án thực tế dài hơn 45–60 phút nếu không có cơ chế giảm án. Server hiện giữ toàn bộ trạng thái trong RAM; restart sẽ mất trạng thái.

## Kiến trúc

- `internal/game`: luật độc lập mạng, tiền, lao động, giảm án, thể lực và xác suất vượt ngục.
- `internal/prison`: một goroutine actor cho mỗi trại; mọi thay đổi trạng thái đi qua channel, tick 2 lần/giây.
- `internal/hub`: chọn trại còn chỗ hoặc tạo trại mới, ánh xạ token tới trại.
- `internal/proto`: giao thức JSON.
- `cmd/server`: HTTP, WebSocket, static embed bằng `go:embed`, gzip cho tài nguyên text. `cmd/server/web` là bản sao được `make` đồng bộ từ `web/` trước build/run/test để đáp ứng giới hạn đường dẫn của `go:embed`.
- `cmd/loadtest`: client WebSocket giả lập, ping latency, p95 và tùy chọn RSS qua `/proc`.

### Mở rộng ngang

Không có shared database nên một người chơi phải luôn được định tuyến về cùng tiến trình. Gắn `prisonId` vào token/session routing, sau đó dùng consistent hash hoặc `hash(prisonId) % số_shard` tại load balancer để đưa mọi kết nối cùng trại về cùng tiến trình. Có thể đặt N tiến trình sau Envoy/HAProxy/Nginx với sticky routing theo prison ID. Không thể tự động di chuyển trại đang chạy giữa tiến trình; cần drain trại hoặc chấp nhận mất phiên khi tiến trình chết.

## Load test

```sh
make loadtest
# tùy chỉnh
 go run ./cmd/loadtest -n 2000 -rate 1 -url ws://127.0.0.1:8080/ws -pid <PID_SERVER>
```

Công cụ tạo kết nối đồng thời trong khoảng ramp-up ngắn, giữ kết nối 10 giây, gửi ping theo tốc độ cấu hình và in số kết nối, độ trễ trung bình/p95. `-pid` đọc RSS từ `/proc` trên Linux. Kết quả phụ thuộc CPU, giới hạn file descriptor và băng thông. Chưa chạy benchmark 2.000 client trong môi trường hiện tại vì không tải được dependency WebSocket; không ghi số liệu benchmark giả định vào README.

## Giới hạn hiện tại

Đây là bản chơi được theo luồng cốt lõi, không phải triển khai production đã kiểm chứng tải 2.000 người. Cần benchmark trên máy mục tiêu trước khi công bố mức tải. Giao dịch chuyển tiền, chat trại/buồng, buồng 6 người, reconnect grace 60 giây, minigame lao động và avatar người chơi cùng khu vực đã có. Bản Social Update bổ sung bạn bè/lời mời, hồ sơ đổi tên và ngoại hình, băng nhóm tối đa 20 thành viên, chat nhóm, nhắn tin riêng, biểu cảm, chợ người chơi, điểm xã hội, bảng xếp hạng và sự kiện toàn trại. Các dữ liệu xã hội hiện vẫn lưu trong RAM theo từng tiến trình/trại; restart có thể làm mất dữ liệu và chưa đồng bộ xuyên các prison shard. Các luật vay lãi, hệ thống nợ, giới hạn giao dịch/phút, lịch tuần cai ngục, bot thay thế người mất kết nối, chuỗi sự kiện có lựa chọn đầy đủ và báo cáo chat còn giản lược. Cai ngục hiện là mô hình tĩnh, chưa có tuần tra AI; các phạm nhân ở khu vực khác nhau chưa được đồng bộ hiển thị toàn cục. Three.js dùng CDN do không thể tải vendor trong môi trường tạo file ngoại tuyến. `SENTENCE_DAYS` là mốc trung tâm; án được ngẫu nhiên trong khoảng mốc ±10 và giới hạn 20–40 ngày.

## Social Update

- **Bạn bè:** gửi/nhận lời mời, chấp nhận, xoá bạn, nhắn tin riêng bằng biệt danh hoặc token người chơi.
- **Băng nhóm:** tạo nhóm, tham gia nhóm đang mở, rời nhóm, chat nhóm; giới hạn 20 thành viên.
- **Chợ người chơi:** đăng bán vật phẩm trong kho, escrow vật phẩm khi đăng, mua và chuyển tiền nguyên tử trong actor của trại, huỷ tin đăng để nhận lại hàng.
- **Tương tác:** biểu cảm chào/cười/tức giận/trái tim/nhảy; cập nhật tên và ngoại hình.
- **Tiến trình xã hội:** điểm xã hội từ kết bạn, tham gia nhóm, giao dịch và biểu cảm; bảng xếp hạng top 10.
- **Sự kiện:** sự kiện cộng đồng ngẫu nhiên được phát tới toàn trại theo chu kỳ.

Lưu ý: đây là bản cập nhật gameplay đầu tiên, chưa phải mạng xã hội production. Bạn bè, nhóm, chợ và điểm vẫn nằm trong RAM; chưa có lưu trữ bền vững, kiểm duyệt/báo cáo người chơi, mời nhóm riêng, thuế chợ, phân trang tin đăng hoặc chống thông đồng giá. Nhắn tin riêng hiện yêu cầu người nhận đang có mặt trong cùng prison shard. Trước khi mở public cần bổ sung persistence, giới hạn tạo nhóm/giao dịch, kiểm duyệt và công cụ quản trị.

## Bảo mật và vận hành

- WebSocket giới hạn payload 4 KiB, text chat 200 ký tự, 8 thao tác/giây/connection.
- Server xác thực phần lớn kết quả hoạt động; điểm minigame và thời gian được kiểm tra ngưỡng.
- Token là định danh ẩn danh ngẫu nhiên lưu localStorage. Không dùng token này như thông tin xác thực cho dữ liệu nhạy cảm.
- Chưa có TLS termination, origin allowlist, distributed rate limiting hay persistence; triển khai Internet cần reverse proxy TLS và bổ sung các lớp này.
