# Cập nhật thế giới 3D

## Điều khiển
- W/A/S/D hoặc phím mũi tên: di chuyển nhân vật.
- Nhấp đúp vào vùng 3D để khóa chuột; di chuyển chuột để xoay hướng nhân vật.
- Nhấp vào nút tên khu vực hoặc nền khu vực để đi tới.
- Trên điện thoại: vuốt ngắn trên vùng 3D để di chuyển.

## Thay đổi
- Chuyển camera từ orthographic/isometric sang camera góc nhìn thứ ba có bám theo nhân vật mượt.
- Thế giới có sân trung tâm, buồng giam có giường/tủ, xưởng lao động, căng tin, phòng gym, lớp học, khu thăm nuôi và cổng.
- Các phòng có tường bao, cửa ra vào, nội thất và va chạm cơ bản; đường đi tự động ưu tiên qua cửa.
- Có đèn, bóng đổ, sương xa và tối ưu chất lượng đồ họa thấp.

## Lưu ý kỹ thuật
- Đồng bộ multiplayer hiện tại của backend cũ mới gửi khu vực (zone), chưa gửi tọa độ/vận tốc liên tục. Vì vậy nhân vật của người khác được đặt gần vị trí đại diện trong khu vực, chưa thể phản ánh chính xác từng bước chân. Muốn đồng bộ chuyển động thật giữa nhiều người chơi cần bổ sung message position + server broadcast/interpolation và kiểm soát tốc độ ở backend.
- Để Render build thành công, chạy `go mod tidy` trên máy có Internet tại thư mục chứa `go.mod`, sau đó commit cả `go.mod` và `go.sum`.
