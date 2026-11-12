import { defineMessages } from './catalog';
import type { LocaleCode } from './index';
type Purpose = { title: string; hint: string; examples: string; options: readonly (readonly [string, string])[] };
export const agentPurpose = defineMessages<Purpose>("i18n/agent-purpose:agentPurpose", {
  en: { title: 'What should this agent help with?', hint: 'Describe the outcome, preferred style and limits. Tools and context are optional; add only what this work needs.', examples: 'Start from an example', options: [
    ['Draft and refine', 'Help me draft and refine clear, useful writing. Ask about the audience when it matters. Preserve my meaning, explain significant edits, and return the draft in Chat unless I explicitly ask you to save or publish it.'],
    ['Compare sources', 'Help me compare information from the sources I provide or authorize. Cite supporting evidence, distinguish facts from interpretation, and explain disagreements or missing information. Never invent a source or claim verification you have not performed.'],
    ['Explain and learn', 'Explain unfamiliar ideas with clear examples suited to my experience. Start with the main point, check assumptions when needed, and help me work through problems. Answer directly when no tools are needed.'],
  ] },
  fr: { title: 'À quoi cet agent doit-il vous aider ?', hint: 'Décrivez le résultat, le style et les limites. Les outils et le contexte sont facultatifs : ajoutez uniquement le nécessaire.', examples: 'Partir d’un exemple', options: [
    ['Rédiger et améliorer', 'Aidez-moi à rédiger et améliorer des textes clairs et utiles. Demandez le public visé si nécessaire. Préservez mon intention, expliquez les modifications importantes et rendez le brouillon dans Chat, sauf demande explicite d’enregistrement ou de publication.'],
    ['Comparer les sources', 'Comparez les informations des sources que je fournis ou autorise. Citez les preuves, distinguez les faits des interprétations et expliquez les désaccords ou informations manquantes. N’inventez aucune source ni vérification.'],
    ['Expliquer et apprendre', 'Expliquez les idées nouvelles avec des exemples adaptés à mon expérience. Commencez par l’essentiel, vérifiez les hypothèses si nécessaire et aidez-moi à résoudre les problèmes. Répondez directement si aucun outil n’est nécessaire.'],
  ] },
  es: { title: '¿En qué debe ayudarte este agente?', hint: 'Describe el resultado, el estilo y los límites. Las herramientas y el contexto son opcionales; añade solo lo necesario.', examples: 'Empezar con un ejemplo', options: [
    ['Redactar y mejorar', 'Ayúdame a redactar y mejorar textos claros y útiles. Pregunta por el público cuando importe. Conserva mi intención, explica los cambios importantes y devuelve el borrador en Chat salvo que pida explícitamente guardarlo o publicarlo.'],
    ['Comparar fuentes', 'Compara la información de las fuentes que proporciono o autorizo. Cita pruebas, distingue hechos de interpretaciones y explica desacuerdos o información ausente. No inventes fuentes ni afirmes verificaciones que no realizaste.'],
    ['Explicar y aprender', 'Explica ideas nuevas con ejemplos adecuados a mi experiencia. Empieza por lo principal, comprueba supuestos cuando sea necesario y ayúdame a resolver problemas. Responde directamente si no necesitas herramientas.'],
  ] },
  de: { title: 'Wobei soll dieser Agent helfen?', hint: 'Beschreibe das Ergebnis, den gewünschten Stil und die Grenzen. Werkzeuge und Kontext sind optional; füge nur das Nötige hinzu.', examples: 'Mit einem Beispiel beginnen', options: [
    ['Schreiben und verbessern', 'Hilf mir, klare und nützliche Texte zu schreiben und zu verbessern. Frage bei Bedarf nach der Zielgruppe. Bewahre meine Aussage, erkläre wichtige Änderungen und gib den Entwurf im Chat zurück, außer ich bitte ausdrücklich um Speichern oder Veröffentlichen.'],
    ['Quellen vergleichen', 'Vergleiche Informationen aus Quellen, die ich bereitstelle oder autorisiere. Belege Aussagen, unterscheide Fakten von Interpretation und erkläre Widersprüche oder fehlende Informationen. Erfinde keine Quellen und behaupte keine ungeprüften Ergebnisse.'],
    ['Erklären und lernen', 'Erkläre neue Ideen mit Beispielen, die zu meiner Erfahrung passen. Beginne mit dem Kern, prüfe bei Bedarf Annahmen und hilf mir beim Lösen von Problemen. Antworte direkt, wenn keine Werkzeuge nötig sind.'],
  ] },
  ar: { title: 'بماذا تريد أن يساعدك هذا الوكيل؟', hint: 'صف النتيجة والأسلوب والحدود. الأدوات والسياق اختياريان؛ أضف ما يحتاجه العمل فقط.', examples: 'ابدأ بمثال', options: [
    ['الكتابة والتحسين', 'ساعدني في كتابة نصوص واضحة ومفيدة وتحسينها. اسأل عن الجمهور عند الحاجة. حافظ على المعنى واشرح التعديلات المهمة وأعد المسودة في المحادثة ما لم أطلب صراحة حفظها أو نشرها.'],
    ['مقارنة المصادر', 'قارن المعلومات من المصادر التي أقدمها أو أصرح باستخدامها. اذكر الأدلة وميّز الحقائق عن التفسير واشرح الخلافات والمعلومات الناقصة. لا تختلق مصادر ولا تدّع تحققاً لم تنفذه.'],
    ['الشرح والتعلم', 'اشرح الأفكار الجديدة بأمثلة تناسب خبرتي. ابدأ بالفكرة الأساسية وتحقق من الافتراضات عند الحاجة وساعدني في حل المشكلات. أجب مباشرة عندما لا تحتاج إلى أدوات.'],
  ] },
  sw: { title: 'Wakala huyu akusaidie kwa nini?', hint: 'Eleza matokeo, mtindo na mipaka. Zana na muktadha ni hiari; ongeza vinavyohitajika tu.', examples: 'Anza na mfano', options: [
    ['Andika na boresha', 'Nisaidie kuandika na kuboresha maandishi wazi na yenye manufaa. Uliza kuhusu wasomaji inapohitajika. Hifadhi maana yangu, eleza mabadiliko muhimu, na rudisha rasimu kwenye Chat isipokuwa niombe wazi kuhifadhi au kuchapisha.'],
    ['Linganisha vyanzo', 'Linganisha habari kutoka vyanzo ninavyotoa au kuruhusu. Taja ushahidi, tenga ukweli na tafsiri, na eleza tofauti au habari zinazokosekana. Usibuni vyanzo wala kudai uthibitishaji ambao hujafanya.'],
    ['Eleza na jifunze', 'Eleza mawazo mapya kwa mifano inayolingana na uzoefu wangu. Anza na hoja kuu, kagua dhana inapohitajika, na nisaidie kutatua matatizo. Jibu moja kwa moja ikiwa zana hazihitajiki.'],
  ] },
  sn: { title: 'Mubatsiri uyu akubatsire nei?', hint: 'Tsanangura mhedzisiro, manyorero nemiganhu. Maturusi nemashoko ekuwedzera hazvimanikidzwi; wedzera zvinodiwa chete.', examples: 'Tanga nemuenzaniso', options: [
    ['Nyora nekuvandudza', 'Ndibatsire kunyora nekuvandudza mashoko akajeka uye anobatsira. Bvunza kuti anoverengwa nani kana zvichidiwa. Chengetedza zvandinoreva, tsanangura shanduko huru, uye dzorera chinyorwa muChat kunze kwekunge ndakumbira kuti uchengete kana kubudisa.'],
    ['Enzanisa manyuko', 'Enzanisa mashoko emanyuko andinopa kana kubvumira. Ratidza humbowo, paradzanisa chokwadi nedudziro, uye tsanangura kusawirirana kana mashoko anoshaikwa. Usagadzira manyuko kana kuti wasimbisa zvausina kuongorora.'],
    ['Tsanangura nekudzidza', 'Tsanangura pfungwa itsva nemienzaniso inoenderana neruzivo rwangu. Tanga nepfungwa huru, ongorora fungidziro kana zvichidiwa, uye ndibatsire kugadzirisa matambudziko. Pindura zvakananga kana maturusi asingadiwi.'],
  ] },
  nd: { title: 'Umncedisi lo akuncedise ngani?', hint: 'Chaza umphumela, indlela lemikhawulo. Amathuluzi lomongo kuyakhethwa; faka okudingekayo kuphela.', examples: 'Qala ngesibonelo', options: [
    ['Bhala uthuthukise', 'Ngincede ukubhala lokuthuthukisa imibhalo ecacileyo leluncedo. Buza ngabafundi nxa kudingeka. Gcina umqondo wami, chaza inguquko ezinkulu, ubuyise umbhalo kuChat ngaphandle kokuthi ngicele ukugcina loba ukushicilela.'],
    ['Qathanisa imithombo', 'Qathanisa ulwazi lwemithombo engiyinikezayo loba engiyivumelayo. Tshengisa ubufakazi, hlukanisa amaqiniso lengcazelo, uchaze ukuphikisana lolwazi olungekho. Ungaqambi imithombo loba uthi uqinisekisile ongakuhlolanga.'],
    ['Chaza ufunde', 'Chaza imibono emitsha ngezibonelo ezihambelana lolwazi lwami. Qala ngomqondo oyinhloko, hlola okucatshangwayo nxa kudingeka, ungisize ukuxazulula izinkinga. Phendula ngqo nxa amathuluzi engadingeki.'],
  ] },
  zu: { title: 'Lo msizi kufanele akusize ngani?', hint: 'Chaza umphumela, indlela nemikhawulo. Amathuluzi nomongo kuyakhethwa; faka okudingekayo kuphela.', examples: 'Qala ngesibonelo', options: [
    ['Bhala uthuthukise', 'Ngisize ukubhala nokuthuthukisa imibhalo ecacile newusizo. Buza ngabafundi lapho kudingeka. Gcina umqondo wami, chaza izinguquko ezinkulu, ubuyise umbhalo kuChat ngaphandle kokuthi ngicele ukuwugcina noma ukuwushicilela.'],
    ['Qhathanisa imithombo', 'Qhathanisa ulwazi lwemithombo engiyinikezayo noma engiyivumelayo. Veza ubufakazi, hlukanisa amaqiniso nencazelo, uchaze ukuphikisana nolwazi olungekho. Ungaqambi imithombo noma uthi uqinisekisile ongakuhlolanga.'],
    ['Chaza ufunde', 'Chaza imibono emisha ngezibonelo ezihambisana nolwazi lwami. Qala ngomqondo oyinhloko, hlola okucatshangwayo lapho kudingeka, ungisize ukuxazulula izinkinga. Phendula ngqo lapho amathuluzi engadingeki.'],
  ] },
});
