# 2PC-Demo
## 2PC可運用的情境
參考 [Triton Ho 大神提供的例子](https://www.facebook.com/groups/616369245163622/permalink/1452268674907004)
### 情境1 (version1 branch)
 我們Passkit公司，向用戶收費時，我們是使用Stripe的金流服務的。<br>
 Step 0: 會計系統為用戶建立計好這個月該付的錢，建立了payment的記錄，其status是"Initial"<br>
 Step 1: 呼叫Stripe API，向用戶收費。<br>
 Step 2: 在Passkit資料庫，update payments set status = "Success" where id = @id;<br>
 這樣寫，在一般情況下是沒有問題的。<br>
 可是嘛，如果我們Passkit伺服器，在完成了第一步，向客戶收費後才當掉～<br>
 那麼我們重新啟動伺服器時。因為我們無法知道這個用戶是在收費後才當掉的，所以系統便會再向用戶收費！<br>
 這樣，用戶便會被收了雙份的錢。<br>
### 情境2 (version2 branch)
 Step 0: 會計系統為用戶建立計好這個月該付的錢，建立了payment的記錄，其status是"Initial"<br>
 Step 1: 在Passkit資料庫，update payments set status = "Success" where id = @id;<br>
 Step 2: 呼叫Stripe API，向用戶收費。<br>
 那麼，如果我們Passkit伺服器在完成了第一步才當掉～<br>
 我們重新啟動伺服器時，我們便看到這筆payment紀錄的狀態是"Success"的，系統便會誤會這筆紀錄是已經收了錢。
 所以系統便不會再理會這筆紀錄，讓用戶沒被收費。
### 情境3 (master branch)
 Step 0: 會計系統為用戶建立計好這個月該付的錢，建立了payment的記錄，其status是"Initial"<br>
 Step 1: 呼叫Stripe API，建立Stripe.Charge的物件，這個Charge的captured屬性為false，所以現在還未向用戶的信用卡收款。<br>
 Step 2: 在Passkit資料庫，把第一步Stripe.Charge物件的id存起來，update payments set stripeChargeId = @Stripe.Charge.Id where id = @id;<br>
 Step 3: 呼叫Stripe API，把第一步建立的Stripe.Charge的captured屬性改為true，向用戶的信用卡收款。<br>
 Step 4: update payments set status = "Success" where id = @id;<br>
 現在我們看看Passkit伺服器當掉後重啟會發生什麼事吧：<br>
 如果在第０－１步之間當掉：<br>
 系統發現某payment記錄的status是"Initial"，而且stripeChargeId為空的，所以從第一步開始重做便好了。<br>
 如果在第１－２步之間當掉：<br>
 系統發現某payment記錄的status是"Initial"，而且stripeChargeId為空的，所以從第一步開始重做。<br>
 雖然在當掉之前已經建立了一個Stripe.Charge物件，不過因為那個Stripe.Charge物件的captured屬性為false，還沒向客戶收款的。所以用戶在整個過程中只會被收一次錢，不會發現Passkit伺服器曾經當掉過。<br>
 如果在第２－３步之間當掉：<br>
 系統發現payment記錄已經有了stripeChargeId，所以重做第３和４步便好。<br>
 如果在第３－４步之間當掉：<br>
 系統發現payment記錄已經有了stripeChargeId，所以重做第３步。<br>
 不過在Stripe系統的Charge.captured之前便已經從false變成了true並且向用戶收款了。現在Charge.captured從true變成true，Stripe是不會再向用戶收費的，所以不會重覆收款。<br>
 簡單來說：<br>
 不管Passkit伺服器在什麼時候當掉，Passkit用戶肯定會被收款，也只會收一次。<br>
 
## How to run?
### Requirement
* PostgreSQL
* Go

### passkit environment variable
* PostgreSQL URL
* Crash (demo crash problem for version1 and version2)

### stripeapi environment variable
nope

