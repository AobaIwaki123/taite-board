import SwiftUI
import TaiteBoardCore

struct NextView: View {
    let snapshot: RepositorySnapshot
    let session: Session?
    let isNextLive: Bool
    let selectedImageIndex: Int
    let onSelectImageIndex: (Int) -> Void
    let onSelectSession: (String) -> Void
    let onResetToNextLive: () -> Void
    let onBackToList: () -> Void

    @State private var isImageViewerPresented = false

    var body: some View {
        ScrollView {
            VStack(alignment: .leading, spacing: 14) {
                if let session {
                    if !isNextLive {
                        selectedSessionBanner
                    }

                    imagePreview(for: session)

                    sameDaySessionPicker(for: session)

                    eventHeader(for: session)

                    HStack(alignment: .top, spacing: 10) {
                        timeCard(
                            title: "出番",
                            detail: session.live.stage ?? "ステージ",
                            start: session.live.startTime,
                            end: session.live.endTime,
                            icon: "mic.fill",
                            color: RibbonTheme.rose
                        )
                        timeCard(
                            title: "特典会",
                            detail: session.benefit.type ?? "終演後",
                            start: session.benefit.startTime,
                            end: session.benefit.endTime,
                            icon: "gift.fill",
                            color: RibbonTheme.purple,
                            footer: session.benefit.booth.map { "ブース: \($0)" }
                        )
                    }

                    details(for: session)
                    ticketAction(for: session)
                    promotions
                } else {
                    ContentUnavailableView(
                        "次回公演はありません",
                        systemImage: "calendar.badge.checkmark",
                        description: Text("表示できる公演が見つかりません。")
                    )
                    .frame(maxWidth: .infinity)
                    .padding(.top, 40)
                }
            }
            .padding(.top, 10)
            .padding(.bottom, 12)
        }
        .scrollIndicators(.hidden)
        .sheet(isPresented: $isImageViewerPresented) {
            if let session {
                RibbonImageViewer(
                    images: displayImages(for: session),
                    selectedIndex: selectedImageIndex,
                    onSelectIndex: onSelectImageIndex
                )
            }
        }
    }

    private var selectedSessionBanner: some View {
        HStack(spacing: 8) {
            Button {
                onBackToList()
            } label: {
                Label("一覧へ戻る", systemImage: "chevron.left")
                    .font(.caption.weight(.medium))
                    .foregroundStyle(RibbonTheme.textPrimary)
            }
            .buttonStyle(.plain)

            Spacer(minLength: 8)

            Button("直近ライブへ") {
                onResetToNextLive()
            }
            .font(.caption.weight(.bold))
            .foregroundStyle(RibbonTheme.background)
            .padding(.horizontal, 9)
            .padding(.vertical, 5)
            .background(RibbonTheme.rose, in: RoundedRectangle(cornerRadius: 8))
            .buttonStyle(.plain)
        }
        .padding(9)
        .background(Color(red: 0.231, green: 0.039, blue: 0.078).opacity(0.62), in: RoundedRectangle(cornerRadius: 12))
        .overlay(RoundedRectangle(cornerRadius: 12).stroke(RibbonTheme.roseStrong.opacity(0.35), lineWidth: 1))
    }

    private func imagePreview(for session: Session) -> some View {
        let images = displayImages(for: session)
        let selectedImage = images.indices.contains(selectedImageIndex) ? images[selectedImageIndex] : images[0]

        return ZStack(alignment: .bottomLeading) {
            ScheduleRemoteImage(source: selectedImage.url, contentMode: .fill)
                .frame(maxWidth: .infinity)
                .frame(height: 176)

            LinearGradient(
                colors: [.clear, Color.black.opacity(0.25)],
                startPoint: .top,
                endPoint: .bottom
            )

            if images.count > 1 {
                HStack(spacing: 6) {
                    ForEach(Array(images.enumerated()), id: \.offset) { index, image in
                        Button {
                            onSelectImageIndex(index)
                        } label: {
                            Label(
                                image.type == "timetable" ? "タイテ表" : "フライヤー",
                                systemImage: image.type == "timetable" ? "calendar" : "photo"
                            )
                            .font(.caption2.weight(.bold))
                            .foregroundStyle(index == selectedImageIndex ? RibbonTheme.background : RibbonTheme.textSecondary)
                            .padding(.horizontal, 10)
                            .padding(.vertical, 6)
                            .background(
                                index == selectedImageIndex ? RibbonTheme.rose : RibbonTheme.surface,
                                in: RoundedRectangle(cornerRadius: 8)
                            )
                            .overlay(
                                RoundedRectangle(cornerRadius: 8)
                                    .stroke(index == selectedImageIndex ? .clear : RibbonTheme.border, lineWidth: 1)
                            )
                        }
                        .buttonStyle(.plain)
                    }
                }
                .padding(10)
            }

            if images.count > 1 {
                Text("\(min(selectedImageIndex + 1, images.count))/\(images.count)")
                    .font(.caption2.monospaced().weight(.bold))
                    .foregroundStyle(RibbonTheme.textPrimary)
                    .padding(.horizontal, 8)
                    .padding(.vertical, 4)
                    .background(RibbonTheme.background.opacity(0.82), in: RoundedRectangle(cornerRadius: 6))
                    .overlay(RoundedRectangle(cornerRadius: 6).stroke(RibbonTheme.border, lineWidth: 1))
                    .padding(10)
                    .frame(maxWidth: .infinity, maxHeight: .infinity, alignment: .topLeading)
            }

            Button {
                isImageViewerPresented = true
            } label: {
                Image(systemName: "arrow.up.left.and.arrow.down.right")
                    .font(.caption.weight(.bold))
                    .foregroundStyle(RibbonTheme.textPrimary)
                    .padding(8)
                    .background(RibbonTheme.background.opacity(0.82), in: RoundedRectangle(cornerRadius: 10))
                    .overlay(RoundedRectangle(cornerRadius: 10).stroke(RibbonTheme.border, lineWidth: 1))
            }
            .buttonStyle(.plain)
            .padding(10)
            .frame(maxWidth: .infinity, maxHeight: .infinity, alignment: .topTrailing)
        }
        .ribbonCard(cornerRadius: 16)
    }

    @ViewBuilder
    private func sameDaySessionPicker(for session: Session) -> some View {
        let sameDaySessions = snapshot.document.sessions.filter { $0.date == session.date }
        if sameDaySessions.count > 1 {
            ScrollView(.horizontal) {
                HStack(spacing: 6) {
                    ForEach(sameDaySessions, id: \.id) { candidate in
                        let isSelected = candidate.id == session.id
                        Button {
                            onSelectSession(candidate.id)
                        } label: {
                            HStack(spacing: 6) {
                                Text(candidate.sessionName ?? String(candidate.title.prefix(10)))
                                Text("\(candidate.live.startTime)~")
                                    .font(.caption2.monospaced())
                                    .opacity(0.8)
                            }
                            .font(.caption.weight(.bold))
                            .foregroundStyle(isSelected ? RibbonTheme.background : RibbonTheme.textSecondary)
                            .padding(.horizontal, 12)
                            .padding(.vertical, 7)
                            .background(
                                isSelected
                                    ? AnyShapeStyle(LinearGradient(colors: [RibbonTheme.rose, RibbonTheme.roseStrong], startPoint: .leading, endPoint: .trailing))
                                    : AnyShapeStyle(RibbonTheme.surface),
                                in: Capsule()
                            )
                            .overlay(Capsule().stroke(isSelected ? .clear : RibbonTheme.border, lineWidth: 1))
                        }
                        .buttonStyle(.plain)
                    }
                }
            }
            .scrollIndicators(.hidden)
        }
    }

    private func eventHeader(for session: Session) -> some View {
        VStack(alignment: .leading, spacing: 10) {
            HStack(alignment: .top, spacing: 8) {
                Label(
                    "\(session.date) (\(session.dayOfWeek ?? ""))",
                    systemImage: "calendar"
                )
                .font(.caption.weight(.bold))
                .foregroundStyle(RibbonTheme.rose)

                Spacer(minLength: 4)

                if isNextLive {
                    Text("★ 直近ライブ")
                        .font(.caption2.weight(.bold))
                        .foregroundStyle(RibbonTheme.rose)
                        .padding(.horizontal, 8)
                        .padding(.vertical, 4)
                        .background(RibbonTheme.rose.opacity(0.15), in: RoundedRectangle(cornerRadius: 5))
                        .overlay(RoundedRectangle(cornerRadius: 5).stroke(RibbonTheme.roseStrong.opacity(0.35), lineWidth: 1))
                }
            }

            Text(session.title)
                .font(.title3.weight(.black))
                .foregroundStyle(RibbonTheme.textPrimary)
                .fixedSize(horizontal: false, vertical: true)

            Divider()
                .overlay(RibbonTheme.border)

            if let mapURL = session.venue.mapUrl, let url = URL(string: mapURL) {
                Link(destination: url) {
                    Label(session.venue.name, systemImage: "mappin.and.ellipse")
                        .font(.caption.weight(.semibold))
                        .foregroundStyle(RibbonTheme.textPrimary)
                        .underline()
                }
            } else {
                Label(session.venue.name, systemImage: "mappin.and.ellipse")
                    .font(.caption.weight(.semibold))
                    .foregroundStyle(RibbonTheme.textSecondary)
            }
        }
        .padding(14)
        .ribbonCard()
    }

    private func timeCard(
        title: String,
        detail: String,
        start: String,
        end: String?,
        icon: String,
        color: Color,
        footer: String? = nil
    ) -> some View {
        VStack(alignment: .leading, spacing: 5) {
            Label(title + " (" + detail + ")", systemImage: icon)
                .font(.caption2.weight(.bold))
                .foregroundStyle(color.opacity(0.95))
                .lineLimit(2)

            Text("\(start)〜\(end ?? "")")
                .font(.title3.monospacedDigit().weight(.black))
                .foregroundStyle(RibbonTheme.textPrimary)
                .minimumScaleFactor(0.75)
                .lineLimit(1)

            if let footer {
                Text(footer)
                    .font(.caption2.weight(.medium))
                    .foregroundStyle(RibbonTheme.textSecondary)
                    .lineLimit(1)
            }
        }
        .frame(maxWidth: .infinity, alignment: .leading)
        .padding(13)
        .overlay(alignment: .leading) {
            Rectangle()
                .fill(color)
                .frame(width: 4)
        }
        .ribbonCard(cornerRadius: 16)
    }

    @ViewBuilder
    private func details(for session: Session) -> some View {
        VStack(spacing: 8) {
            if let rule = session.photo?.rule {
                detailStrip(icon: "camera.aperture", title: "撮影可否：", text: rule, color: .green)
            }

            if let admissionBonus = session.admissionBonus {
                detailStrip(icon: "gift.fill", title: "入場特典：", text: admissionBonus, color: RibbonTheme.rose)
            }

            if let prices = session.ticket?.prices, !prices.isEmpty {
                VStack(alignment: .leading, spacing: 8) {
                    Label("チケット料金", systemImage: "ticket.fill")
                        .font(.caption.weight(.bold))
                        .foregroundStyle(RibbonTheme.textSecondary)

                    LazyVGrid(columns: [GridItem(.adaptive(minimum: 128), alignment: .leading)], alignment: .leading, spacing: 6) {
                        ForEach(Array(prices.enumerated()), id: \.offset) { _, price in
                            HStack(spacing: 4) {
                                Text(price.tier ?? "")
                                    .foregroundStyle(RibbonTheme.textSecondary)
                                Text(price.price ?? "")
                                    .fontWeight(.bold)
                                    .foregroundStyle(RibbonTheme.rose)
                            }
                            .font(.caption2)
                            .padding(.horizontal, 8)
                            .padding(.vertical, 6)
                            .background(RibbonTheme.background, in: RoundedRectangle(cornerRadius: 8))
                            .overlay(RoundedRectangle(cornerRadius: 8).stroke(RibbonTheme.border, lineWidth: 1))
                        }
                    }
                }
                .padding(12)
                .ribbonCard(cornerRadius: 12)
            }
        }
    }

    private func detailStrip(icon: String, title: String, text: String, color: Color) -> some View {
        HStack(alignment: .top, spacing: 8) {
            Image(systemName: icon)
                .font(.subheadline)
                .foregroundStyle(color)
                .frame(width: 18)
            Text(LocalizedStringKey(title))
                .fontWeight(.bold)
                .foregroundStyle(color.opacity(0.95))
            + Text(text)
                .foregroundStyle(RibbonTheme.textSecondary)
        }
        .font(.caption)
        .frame(maxWidth: .infinity, alignment: .leading)
        .padding(12)
        .background(RibbonTheme.surface.opacity(0.82), in: RoundedRectangle(cornerRadius: 12))
        .overlay(RoundedRectangle(cornerRadius: 12).stroke(RibbonTheme.border, lineWidth: 1))
    }

    @ViewBuilder
    private func ticketAction(for session: Session) -> some View {
        if let ticket = session.ticket,
           let urlString = ticket.url,
           let url = URL(string: urlString),
           ticket.platform != "公式アナウンス待ち" {
            Link(destination: url) {
                Label("チケット購入 (\(ticket.platform ?? "外部サイト"))", systemImage: "ticket.fill")
                    .font(.subheadline.weight(.black))
                    .foregroundStyle(RibbonTheme.background)
                    .frame(maxWidth: .infinity)
                    .padding(.vertical, 13)
                    .background(
                        LinearGradient(colors: [RibbonTheme.rose, RibbonTheme.roseStrong], startPoint: .leading, endPoint: .trailing),
                        in: RoundedRectangle(cornerRadius: 12)
                    )
            }
        } else {
            Label("チケット告知待ち", systemImage: "ticket.fill")
                .font(.subheadline.weight(.bold))
                .foregroundStyle(RibbonTheme.textSecondary)
                .frame(maxWidth: .infinity)
                .padding(.vertical, 13)
                .background(RibbonTheme.surfaceElevated, in: RoundedRectangle(cornerRadius: 12))
                .overlay(RoundedRectangle(cornerRadius: 12).stroke(RibbonTheme.border, lineWidth: 1))
        }
    }

    @ViewBuilder
    private var promotions: some View {
        let activePromotions = (snapshot.document.promotions ?? [])
            .filter { $0.active != false }
            .sorted { ($0.priority ?? 0) > ($1.priority ?? 0) }

        ForEach(activePromotions, id: \.id) { promotion in
            PromotionCard(promotion: promotion)
        }
    }

    private func displayImages(for session: Session) -> [DisplayImage] {
        if let images = session.images, !images.isEmpty {
            return images.map { DisplayImage(type: $0.type, label: $0.label, url: $0.url) }
        }
        return [DisplayImage(type: "flyer", label: "フライヤー", url: session.imageUrl ?? "")]
    }
}

private struct DisplayImage: Identifiable {
    let type: String
    let label: String?
    let url: String

    var id: String { "\(type)-\(url)" }
}

struct ScheduleRemoteImage: View {
    let source: String
    let contentMode: ContentMode

    var body: some View {
        AsyncImage(url: resolvedURL) { phase in
            switch phase {
            case .empty:
                ZStack {
                    RibbonTheme.background
                    ProgressView()
                        .tint(RibbonTheme.rose)
                }
            case let .success(image):
                image
                    .resizable()
                    .aspectRatio(contentMode: contentMode)
            case .failure:
                placeholder
            @unknown default:
                placeholder
            }
        }
        .frame(maxWidth: .infinity, maxHeight: .infinity)
        .background(RibbonTheme.background)
        .clipped()
    }

    private var resolvedURL: URL? {
        if let url = URL(string: source), url.scheme != nil {
            return url
        }

        let path = source as NSString
        let name = path.deletingPathExtension as NSString
        let resourceName = name.lastPathComponent
        let resourceDirectory = name.deletingLastPathComponent
        return Bundle.main.url(
            forResource: resourceName,
            withExtension: path.pathExtension.isEmpty ? nil : path.pathExtension,
            subdirectory: resourceDirectory.isEmpty ? nil : resourceDirectory
        )
    }

    private var placeholder: some View {
        VStack(spacing: 6) {
            Image(systemName: "photo")
                .font(.title3)
            Text("画像なし")
                .font(.caption2)
        }
        .foregroundStyle(RibbonTheme.textSecondary)
    }
}

private struct RibbonImageViewer: View {
    let images: [DisplayImage]
    let selectedIndex: Int
    let onSelectIndex: (Int) -> Void

    @Environment(\.dismiss) private var dismiss

    var body: some View {
        ZStack {
            Color.black.ignoresSafeArea()

            if images.indices.contains(selectedIndex) {
                ScheduleRemoteImage(source: images[selectedIndex].url, contentMode: .fit)
                    .padding(.horizontal, 12)
            }
        }
        .overlay(alignment: .topTrailing) {
            Button {
                dismiss()
            } label: {
                Image(systemName: "xmark")
                    .font(.headline.weight(.bold))
                    .foregroundStyle(.white)
                    .padding(12)
                    .background(Color.white.opacity(0.14), in: Circle())
            }
            .buttonStyle(.plain)
            .padding()
        }
        .overlay(alignment: .bottom) {
            if images.count > 1 {
                HStack(spacing: 8) {
                    ForEach(Array(images.enumerated()), id: \.offset) { index, image in
                        Button(image.type == "timetable" ? "タイテ表" : "フライヤー") {
                            onSelectIndex(index)
                        }
                        .font(.caption.weight(.bold))
                        .foregroundStyle(index == selectedIndex ? RibbonTheme.background : .white)
                        .padding(.horizontal, 12)
                        .padding(.vertical, 8)
                        .background(index == selectedIndex ? RibbonTheme.rose : Color.white.opacity(0.14), in: Capsule())
                    }
                }
                .padding(.bottom, 28)
            }
        }
    }
}

private struct PromotionCard: View {
    let promotion: Promotion

    var body: some View {
        VStack(alignment: .leading, spacing: 10) {
            if let imageURL = promotion.imageUrl, !imageURL.isEmpty {
                ScheduleRemoteImage(source: imageURL, contentMode: .fill)
                    .frame(height: 150)
            }

            HStack {
                Text(promotion.badge ?? "お知らせ")
                    .font(.caption2.weight(.bold))
                    .foregroundStyle(.white)
                    .padding(.horizontal, 8)
                    .padding(.vertical, 5)
                    .background(RibbonTheme.roseStrong, in: RoundedRectangle(cornerRadius: 6))
                Spacer()
                Text(promotion.deadline.map { "\($0)まで" } ?? "受付中")
                    .font(.caption2.weight(.bold))
                    .foregroundStyle(RibbonTheme.rose)
            }

            Text(promotion.title)
                .font(.subheadline.weight(.black))
                .foregroundStyle(RibbonTheme.textPrimary)
            if let subtitle = promotion.subtitle {
                Text(subtitle)
                    .font(.caption)
                    .foregroundStyle(RibbonTheme.textSecondary)
            }

            if let url = URL(string: promotion.ctaUrl) {
                Link(destination: url) {
                    Text(promotion.ctaLabel ?? "詳細を見る ↗")
                        .font(.caption.weight(.black))
                        .foregroundStyle(.white)
                        .frame(maxWidth: .infinity)
                        .padding(.vertical, 10)
                        .background(RibbonTheme.surfaceElevated, in: RoundedRectangle(cornerRadius: 10))
                        .overlay(RoundedRectangle(cornerRadius: 10).stroke(RibbonTheme.border, lineWidth: 1))
                }
            }
        }
        .padding(14)
        .ribbonCard(cornerRadius: 16)
    }
}
