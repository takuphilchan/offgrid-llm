function toggleMobileMore() {
    document.getElementById('mobileMoreMenu').classList.toggle('hidden');
}

function hideMobileMore() {
    document.getElementById('mobileMoreMenu').classList.add('hidden');
}

const originalSwitchTabMobile = switchTab;
switchTab = function(tab) {
    originalSwitchTabMobile(tab);
    document.querySelectorAll('.mobile-nav-item').forEach(el => el.classList.remove('active'));
    const mobileTab = document.getElementById('mobile-tab-' + tab);
    if (mobileTab) mobileTab.classList.add('active');
    hideMobileMore();
};

if ('serviceWorker' in navigator) {
    navigator.serviceWorker.register('/ui/sw.js')
        .then(reg => console.log('SW registered:', reg.scope))
        .catch(err => console.log('SW registration failed:', err));
}
