'use strict';
// Accessible terminal labels are interface copy, not shell output.
(function(root){
 const copy={
  "en":["Terminal input","Too much output to read automatically. Navigate the terminal rows to read it.","The terminal request could not be confirmed. Check its state before trying again.","Technical details","Back to workspace"],
  "fr":["Saisie du terminal","Trop de sortie pour une lecture automatique. Parcourez les lignes du terminal pour la lire.","La demande au terminal n’a pas pu être confirmée. Vérifiez son état avant de réessayer.","Détails techniques","Retour à l’espace de travail"],
  "es":["Entrada del terminal","Hay demasiada salida para leerla automáticamente. Recorre las líneas del terminal para leerla.","No se pudo confirmar la solicitud al terminal. Comprueba su estado antes de volver a intentarlo.","Detalles técnicos","Volver al espacio de trabajo"],
  "de":["Terminaleingabe","Zu viel Ausgabe zum automatischen Vorlesen. Navigiere durch die Terminalzeilen, um sie zu lesen.","Die Terminalanfrage konnte nicht bestätigt werden. Prüfe den Zustand vor einem erneuten Versuch.","Technische Details","Zurück zum Arbeitsbereich"],
  "ar":["إدخال الطرفية","المخرجات كثيرة جداً لقراءتها تلقائياً. تنقّل بين أسطر الطرفية لقراءتها.","تعذّر تأكيد طلب الطرفية. تحقّق من حالتها قبل المحاولة مجددًا.","التفاصيل التقنية","العودة إلى مساحة العمل"],
  "sw":["Ingizo la terminal","Matokeo ni mengi mno kusomwa kiotomatiki. Pitia mistari ya terminal ili kuyasoma.","Ombi la terminal halikuweza kuthibitishwa. Angalia hali yake kabla ya kujaribu tena.","Maelezo ya kiufundi","Rudi kwenye eneo la kazi"],
  "sn":["Zvinonyorwa muterminal","Zvabuda zvakawandisa kuti zviverengwe zvoga. Famba mumitsara yeterminal kuti uzviverenge.","Chikumbiro cheterminal hachina kukwanisa kusimbiswa. Tarisa mamiriro acho usati waedza zvakare.","Ruzivo rwehunyanzvi","Dzokera kunzvimbo yebasa"],
  "nd":["Okubhalwa kutheminali","Imiphumela minengi kakhulu ukuthi ifundwe ngokuzenzakalela. Hamba emigqeni yetheminali ukuze uyifunde.","Isicelo setheminali asikwazanga ukuqinisekiswa. Hlola isimo sayo ungakazami futhi.","Imininingwane yobuchwepheshe","Buyela endaweni yokusebenza"],
  "zu":["Okufakwa kutheminali","Imiphumela miningi kakhulu ukuba ifundwe ngokuzenzakalelayo. Hamba emigqeni yetheminali ukuze uyifunde.","Isicelo setheminali asikwazanga ukuqinisekiswa. Hlola isimo sayo ngaphambi kokuzama futhi.","Imininingwane yobuchwepheshe","Buyela endaweni yokusebenza"],
  "pt-BR":["Entrada do terminal","Há saída demais para leitura automática. Navegue pelas linhas do terminal para lê-la.","Não foi possível confirmar a solicitação ao terminal. Verifique o estado dele antes de tentar novamente.","Detalhes técnicos","Voltar ao espaço de trabalho"],
  "pt-PT":["Entrada do terminal","Há demasiada saída para leitura automática. Navegue pelas linhas do terminal para a ler.","Não foi possível confirmar o pedido ao terminal. Verifique o estado do terminal antes de tentar novamente.","Detalhes técnicos","Voltar ao espaço de trabalho"],
  "it":["Input del terminale","Troppo output per la lettura automatica. Scorri le righe del terminale per leggerlo.","Non è stato possibile confermare la richiesta al terminale. Verificane lo stato prima di riprovare.","Dettagli tecnici","Torna allo spazio di lavoro"],
  "ru":["Ввод в терминале","Слишком много вывода для автоматического чтения. Перемещайтесь по строкам терминала, чтобы прочитать его.","Не удалось подтвердить запрос к терминалу. Проверьте его состояние перед повторной попыткой.","Технические сведения","Вернуться в рабочее пространство"],
  "uk":["Введення в терміналі","Забагато виводу для автоматичного читання. Переміщуйтеся рядками термінала, щоб прочитати його.","Не вдалося підтвердити запит до термінала. Перевірте його стан перед повторною спробою.","Технічні відомості","Повернутися до робочого простору"],
  "tr":["Terminal girişi","Otomatik okuma için çok fazla çıktı var. Okumak için terminal satırlarında gezinin.","Terminal isteği doğrulanamadı. Yeniden denemeden önce durumunu kontrol edin.","Teknik ayrıntılar","Çalışma alanına dön"],
  "zh-Hans":["终端输入","输出过多，无法自动朗读。请浏览终端各行以阅读内容。","无法确认终端请求是否完成。请先检查终端状态，再决定是否重试。","技术详情","返回工作区"],
  "zh-Hant":["終端機輸入","輸出過多，無法自動朗讀。請瀏覽終端機各行以閱讀內容。","無法確認終端機請求是否完成。請先檢查終端機狀態，再決定是否重試。","技術詳細資料","返回工作區"],
  "ja":["ターミナル入力","出力が多すぎるため自動で読み上げられません。ターミナルの各行を移動して読んでください。","ターミナルへの要求を確認できませんでした。再試行する前に状態を確認してください。","技術的な詳細","ワークスペースに戻る"],
  "ko":["터미널 입력","출력이 너무 많아 자동으로 읽을 수 없습니다. 터미널의 각 줄을 탐색하여 읽으세요.","터미널 요청을 확인하지 못했습니다. 다시 시도하기 전에 상태를 확인하세요.","기술 세부 정보","작업 공간으로 돌아가기"],
  "hi":["टर्मिनल इनपुट","अपने-आप पढ़ने के लिए आउटपुट बहुत अधिक है। पढ़ने के लिए टर्मिनल की पंक्तियों में जाएँ।","टर्मिनल अनुरोध की पुष्टि नहीं हो सकी। फिर से कोशिश करने से पहले उसकी स्थिति जाँचें।","तकनीकी विवरण","कार्यक्षेत्र पर वापस जाएँ"],
  "bn":["টার্মিনাল ইনপুট","স্বয়ংক্রিয়ভাবে পড়ার জন্য আউটপুট অনেক বেশি। পড়তে টার্মিনালের লাইনগুলোতে যান।","টার্মিনালের অনুরোধ নিশ্চিত করা যায়নি। আবার চেষ্টা করার আগে এর অবস্থা দেখুন।","প্রযুক্তিগত বিবরণ","কর্মক্ষেত্রে ফিরে যান"],
  "ta":["முனைய உள்ளீடு","தானாக வாசிக்க முடியாத அளவு வெளியீடு உள்ளது. வாசிக்க முனைய வரிகளில் நகரவும்.","முனையக் கோரிக்கையை உறுதிப்படுத்த முடியவில்லை. மீண்டும் முயல்வதற்கு முன் அதன் நிலையைச் சரிபார்க்கவும்.","தொழில்நுட்ப விவரங்கள்","பணியிடத்திற்குத் திரும்பு"],
  "ur":["ٹرمینل ان پٹ","خودکار طور پر پڑھنے کے لیے آؤٹ پٹ بہت زیادہ ہے۔ پڑھنے کے لیے ٹرمینل کی سطروں میں جائیں۔","ٹرمینل کی درخواست کی تصدیق نہیں ہو سکی۔ دوبارہ کوشش کرنے سے پہلے اس کی حالت دیکھیں۔","تکنیکی تفصیلات","ورک اسپیس پر واپس جائیں"],
  "fa":["ورودی پایانه","خروجی برای خواندن خودکار بیش از حد زیاد است. برای خواندن آن میان سطرهای پایانه حرکت کنید.","درخواست پایانه تأیید نشد. پیش از تلاش دوباره، وضعیت آن را بررسی کنید.","جزئیات فنی","بازگشت به فضای کاری"],
  "he":["קלט למסוף","יש יותר מדי פלט לקריאה אוטומטית. עברו בין שורות המסוף כדי לקרוא אותו.","לא ניתן היה לאשר את הבקשה למסוף. יש לבדוק את מצבו לפני שמנסים שוב.","פרטים טכניים","חזרה לסביבת העבודה"],
  "id":["Masukan terminal","Keluaran terlalu banyak untuk dibaca otomatis. Telusuri baris terminal untuk membacanya.","Permintaan terminal tidak dapat dikonfirmasi. Periksa statusnya sebelum mencoba lagi.","Detail teknis","Kembali ke ruang kerja"],
  "vi":["Nhập liệu terminal","Có quá nhiều đầu ra để tự động đọc. Hãy di chuyển qua các dòng terminal để đọc.","Không thể xác nhận yêu cầu gửi đến thiết bị đầu cuối. Hãy kiểm tra trạng thái trước khi thử lại.","Chi tiết kỹ thuật","Quay lại không gian làm việc"],
  "th":["อินพุตเทอร์มินัล","เอาต์พุตมากเกินกว่าจะอ่านอัตโนมัติ โปรดเลื่อนดูแต่ละบรรทัดในเทอร์มินัลเพื่ออ่าน","ไม่สามารถยืนยันคำขอของเทอร์มินัลได้ โปรดตรวจสอบสถานะก่อนลองอีกครั้ง","รายละเอียดทางเทคนิค","กลับไปยังพื้นที่ทำงาน"],
  "fil":["Input ng terminal","Masyadong marami ang output para awtomatikong basahin. Lumipat sa mga linya ng terminal upang basahin ito.","Hindi makumpirma ang kahilingan sa terminal. Suriin ang katayuan nito bago subukang muli.","Mga teknikal na detalye","Bumalik sa workspace"]
 };
 if(typeof module==='object'&&module.exports)module.exports=copy;else root.terminalAccessibility=copy;
})(globalThis);
