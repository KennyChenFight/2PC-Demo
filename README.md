 > （吐嘈：為何我有一種從冬眠中被人吵醒的感覺）
 （認真：覺得這篇文對你有幫忙，請在寒冬中餵一下已經絕育的小貓咪，或支持對流浪貓的「捕捉，絕育，再放生」行動。
 今天我想談的是：2 phase commit（以下簡稱2pc）
 詳情可以看：https://en.wikipedia.org/wiki/Two-phase_commit_protocol
 簡單來說。。。。。。
 以上的網頁，正常人類看不明白的，我們直接看實際例子好了。
 （吐嘈：所以要喵和兔子才明白？？？）
 簡單來說：
 我們Passkit公司，向用戶收費時，我們是使用Stripe的金流服務的。
 如果某笨喵這麼寫：
 Step 0: 會計系統為用戶建立計好這個月該付的錢，建立了payment的記錄，其status是"Initial"
 Step 1: 呼叫Stripe API，向用戶收費。
 Step 2: 在Passkit資料庫，update payments set status = "Success" where id = @id;
 這樣寫，在一般情況下是沒有問題的。
 可是嘛，如果我們Passkit伺服器，在完成了第一步，向客戶收費後才當掉～
 那麼我們重新啟動伺服器時。因為我們無法知道這個用戶是在收費後才當掉的，所以系統便會再向用戶收費！
 這樣，用戶便會被收了雙份的錢。（然後某喵便被拿去人道毀滅）
 有人問：那麼，我們把第一步和第二步反過來寫，變成：
 Step 0: 會計系統為用戶建立計好這個月該付的錢，建立了payment的記錄，其status是"Initial"
 Step 1: 在Passkit資料庫，update payments set status = "Success" where id = @id;
 Step 2: 呼叫Stripe API，向用戶收費。
 那麼，如果我們Passkit伺服器在完成了第一步才當掉～
 我們重新啟動伺服器時，我們便看到這筆payment紀錄的狀態是"Success"的，系統便會誤會這筆紀錄是已經收了錢。
 所以系統便不會再理會這筆紀錄，讓用戶沒被收費。（然後Passkit老闆便會把某喵拿去人道毀滅）
 －－－－－－－－－－－－－－－－－－－－－－－－－－－－－－－－－－－－－－－－－－－－－－－－
 以我觀察：
 30%的人真的對2pc完全沒有概念，所以會作出了用戶被收兩次錢／不被收錢的系統。
 65%的人雖然不知道2pc這個名字，但是早便用了2pc的概念，所以他的用戶只會被收一次錢。
 5%的人真的知道2pc，也有好好的活用。
 我們先看看某喵在Passkit作了什麼，讓他不用被拿去人道毀滅吧。
 Step 0: 會計系統為用戶建立計好這個月該付的錢，建立了payment的記錄，其status是"Initial"
 Step 1: 呼叫Stripe API，建立Stripe.Charge的物件，這個Charge的captured屬性為false，所以現在還未向用戶的信用卡收款。
 Step 2: 在Passkit資料庫，把第一步Stripe.Charge物件的id存起來，update payments set stripeChargeId = @Stripe.Charge.Id where id = @id;
 Step 3: 呼叫Stripe API，把第一步建立的Stripe.Charge的captured屬性改為true，向用戶的信用卡收款。
 Step 4: update payments set status = "Success" where id = @id;
 現在我們看看Passkit伺服器當掉後重啟會發生什麼事吧：
 如果在第０－１步之間當掉：
 系統發現某payment記錄的status是"Initial"，而且stripeChargeId為空的，所以從第一步開始重做便好了。
 如果在第１－２步之間當掉：
 系統發現某payment記錄的status是"Initial"，而且stripeChargeId為空的，所以從第一步開始重做。
 雖然在當掉之前已經建立了一個Stripe.Charge物件，不過因為那個Stripe.Charge物件的captured屬性為false，還沒向客戶收款的。所以用戶在整個過程中只會被收一次錢，不會發現Passkit伺服器曾經當掉過。
 如果在第２－３步之間當掉：
 系統發現payment記錄已經有了stripeChargeId，所以重做第３和４步便好。
 如果在第３－４步之間當掉：
 系統發現payment記錄已經有了stripeChargeId，所以重做第３步。
 不過在Stripe系統的Charge.captured之前便已經從false變成了true並且向用戶收款了。現在Charge.captured從true變成true，Stripe是不會再向用戶收費的，所以不會重覆收款。
 簡單來說：
 不管Passkit伺服器在什麼時候當掉，Passkit用戶肯定會被收款，也只會收一次。
 －－－－－－－－－－－－－－－－－－－－－－－－－－－－－－－－－－－－－－－－－－－－－－－－
 眾人問：為什麼你舉的例子，好像跟wikipedia中的2pc看起來差那麼遠？？？
 我：
 （最近爆肝爆太大，為了自己健康，還是先下潛好了。先止筆至此，有空再補完。）
 （註：其實本來是想從2pc談到parse.com的先天性問題的，不過時間不夠。）