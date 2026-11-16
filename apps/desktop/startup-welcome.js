'use strict';
// Shared, local-only startup copy. Draft translations remain subject to human review.
(() => {
  const messages = {
    en: ['Welcome to OffGrid.', 'Bring your ideas to life.', 'Your next task starts here.'],
    fr: ['Bienvenue dans OffGrid.', 'Donnez vie à vos idées.', 'Votre prochaine tâche commence ici.'],
    es: ['Te damos la bienvenida a OffGrid.', 'Da vida a tus ideas.', 'Tu próxima tarea empieza aquí.'],
    ar: ['مرحبًا بك في OffGrid.', 'حوّل أفكارك إلى واقع.', 'مهمتك التالية تبدأ هنا.'],
    sw: ['Karibu OffGrid.', 'Geuza mawazo yako kuwa vitendo.', 'Kazi yako inayofuata inaanzia hapa.'],
    sn: ['Tinokugamuchirai kuOffGrid.', 'Shandurai pfungwa dzenyu kuti dzive zvinhu zvinooneka.', 'Basa renyu rinotevera rinotangira pano.'],
    nd: ['Siyakwamukela ku-OffGrid.', 'Yenza imicabango yakho ibe ngokoqobo.', 'Umsebenzi wakho olandelayo uqala lapha.'],
    zu: ['Siyakwamukela ku-OffGrid.', 'Yenza imibono yakho ibe ngokoqobo.', 'Umsebenzi wakho olandelayo uqala lapha.'],
    de: ['Willkommen bei OffGrid.', 'Erwecke deine Ideen zum Leben.', 'Deine nächste Aufgabe beginnt hier.'],
    'pt-BR': ['Boas-vindas ao OffGrid.', 'Dê vida às suas ideias.', 'Sua próxima tarefa começa aqui.'],
    'pt-PT': ['Bem-vindo ao OffGrid.', 'Dê vida às suas ideias.', 'A sua próxima tarefa começa aqui.'],
    it: ['Benvenuto in OffGrid.', 'Dai vita alle tue idee.', 'Il tuo prossimo lavoro inizia qui.'],
    ru: ['Добро пожаловать в OffGrid.', 'Воплощайте свои идеи в жизнь.', 'Ваша следующая задача начинается здесь.'],
    uk: ['Вітаємо в OffGrid.', 'Втілюйте свої ідеї в життя.', 'Ваше наступне завдання починається тут.'],
    tr: ['OffGrid’e hoş geldiniz.', 'Fikirlerinizi hayata geçirin.', 'Bir sonraki göreviniz burada başlıyor.'],
    'zh-Hans': ['欢迎使用 OffGrid。', '让想法成为现实。', '从这里开始下一项任务。'],
    'zh-Hant': ['歡迎使用 OffGrid。', '讓想法成為現實。', '從這裡開始下一項任務。'],
    ja: ['OffGrid へようこそ。', 'アイデアを形にしましょう。', '次の作業はここから。'],
    ko: ['OffGrid에 오신 것을 환영합니다.', '아이디어를 현실로 만들어 보세요.', '다음 작업을 여기서 시작하세요.'],
    hi: ['OffGrid में आपका स्वागत है।', 'अपने विचारों को साकार करें।', 'आपका अगला काम यहीं से शुरू होता है।'],
    bn: ['OffGrid-এ স্বাগতম।', 'আপনার ভাবনাকে বাস্তবে রূপ দিন।', 'আপনার পরবর্তী কাজ এখান থেকেই শুরু।'],
    ta: ['OffGrid உங்களை வரவேற்கிறது.', 'உங்கள் யோசனைகளை நனவாக்குங்கள்.', 'உங்கள் அடுத்த பணி இங்கிருந்து தொடங்குகிறது.'],
    ur: ['OffGrid میں خوش آمدید۔', 'اپنے خیالات کو حقیقت بنائیں۔', 'آپ کا اگلا کام یہیں سے شروع ہوتا ہے۔'],
    fa: ['به OffGrid خوش آمدید.', 'ایده‌هایتان را به واقعیت تبدیل کنید.', 'کار بعدی شما از اینجا آغاز می‌شود.'],
    he: ['ברוכים הבאים ל-OffGrid.', 'הפכו את הרעיונות שלכם למציאות.', 'המשימה הבאה שלכם מתחילה כאן.'],
    id: ['Selamat datang di OffGrid.', 'Wujudkan ide Anda.', 'Tugas Anda berikutnya dimulai di sini.'],
    vi: ['Chào mừng bạn đến với OffGrid.', 'Biến ý tưởng thành hiện thực.', 'Bắt đầu công việc tiếp theo tại đây.'],
    th: ['ยินดีต้อนรับสู่ OffGrid', 'เปลี่ยนไอเดียให้เป็นจริง', 'เริ่มงานถัดไปได้ที่นี่'],
    fil: ['Maligayang pagdating sa OffGrid.', 'Gawing realidad ang iyong mga ideya.', 'Dito magsisimula ang susunod mong gawain.']
  };

  function create(element, initialLocale, initialOverrides) {
    let locale, copy, index = 0, fade, timer, disposed = false, paused = false;
    const reduced = () => document.documentElement.dataset.reduceMotion === 'true' || matchMedia('(prefers-reduced-motion:reduce)').matches;
    function cancelFade() { clearTimeout(fade); element.classList.remove('is-changing'); }
    function advance() {
      if (disposed || paused || document.hidden) return;
      const show = () => {
        if (disposed) return;
        index = (index + 1) % copy.length;
        element.textContent = copy[index];
        element.classList.remove('is-changing');
      };
      if (reduced()) show();
      else { element.classList.add('is-changing'); fade = setTimeout(show, 200); }
    }
    function schedule() {
      clearInterval(timer); cancelFade();
      if (!disposed && !paused && !document.hidden) timer = setInterval(advance, 5200);
    }
    function setLocale(value, overrides) {
      const next = Object.hasOwn(messages,value) ? value : 'en';
      if (disposed) return;
      const nextCopy = messages[next].map((text,i) => typeof overrides?.[i] === 'string' && overrides[i].trim() ? overrides[i] : text);
      if (next === locale && JSON.stringify(nextCopy) === JSON.stringify(copy)) return;
      cancelFade(); locale = next; copy = nextCopy; index = 0;
      element.textContent = copy[0];
    }
    function dispose() { disposed = true; clearInterval(timer); cancelFade(); document.removeEventListener('visibilitychange',schedule); }
    setLocale(initialLocale,initialOverrides); schedule();
    document.addEventListener('visibilitychange',schedule);
    return {setLocale, setPaused(value) { if (paused === value) return; paused = value; schedule(); }, dispose};
  }
  if (typeof module !== 'undefined' && module.exports) module.exports = {messages,create};
  else window.OffGridStartupWelcome = {create};
})();
