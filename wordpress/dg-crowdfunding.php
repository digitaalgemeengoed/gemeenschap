<?php
/**
 * Plugin Name: Digitaal GemeenGoed — crowdfunding
 * Description: Gemeenten geven op de site een intentie door om bij te dragen aan een project. De kerngroep zet een intentie op "bevestigd" zodra de samenwerkingsovereenkomst is getekend. Shortcode [dg_campagne], beheer onder Toezeggingen.
 *
 * Een campagne staat als kaart in inhoud/ (regels campagne:, doel: en
 * einddatum:, zie inhoud/README.md). De toezeggingen staan in de tabel
 * {prefix}dg_toezeggingen; elke wijziging komt ook als activiteit bij de
 * gemeente in CiviCRM (als CiviCRM aanstaat).
 *
 * Wie mag toezeggen: een ingelogd lid dat via Keycloak aan een organisatie is
 * gekoppeld (Organizations, op e-maildomein). Zelf ingevulde organisaties
 * tellen niet: anders kan iedereen namens elke gemeente toezeggen.
 *
 * @package DigitaalGemeenGoed
 */

defined( 'ABSPATH' ) || exit;

const DG_CF_DB_VERSIE = 1;
const DG_CF_STATUSSEN = array(
	'intentie'    => 'Intentie',
	'bevestigd'   => 'Bevestigd',
	'ingetrokken' => 'Ingetrokken',
	'vervallen'   => 'Vervallen',
);

function dg_cf_tabel(): string {
	global $wpdb;
	return $wpdb->prefix . 'dg_toezeggingen';
}

/** De tabel aanmaken of bijwerken (wp dg inrichten, en bij het eerste gebruik). */
function dg_cf_installeer(): void {
	global $wpdb;
	require_once ABSPATH . 'wp-admin/includes/upgrade.php';
	$tabel = dg_cf_tabel();
	$tekenset = $wpdb->get_charset_collate();
	dbDelta(
		"CREATE TABLE $tabel (
			id bigint(20) unsigned NOT NULL AUTO_INCREMENT,
			campagne varchar(80) NOT NULL,
			organisatie varchar(80) NOT NULL,
			organisatie_naam varchar(190) NOT NULL,
			bedrag int(11) unsigned NOT NULL DEFAULT 0,
			voorwaarde tinyint(1) NOT NULL DEFAULT 0,
			bevoegde_naam varchar(190) NOT NULL DEFAULT '',
			bevoegde_functie varchar(190) NOT NULL DEFAULT '',
			status varchar(20) NOT NULL DEFAULT 'intentie',
			gebruiker bigint(20) unsigned NOT NULL DEFAULT 0,
			aangemaakt datetime NOT NULL,
			gewijzigd datetime NOT NULL,
			bevestigd_op date DEFAULT NULL,
			kenmerk varchar(190) NOT NULL DEFAULT '',
			PRIMARY KEY  (id),
			KEY campagne (campagne),
			KEY organisatie (organisatie)
		) $tekenset;"
	);
	update_option( 'dg_cf_db', DG_CF_DB_VERSIE );
}
add_action(
	'init',
	static function () {
		if ( (int) get_option( 'dg_cf_db' ) !== DG_CF_DB_VERSIE ) {
			dg_cf_installeer();
		}
	}
);

/** Mag de huidige gebruiker toezeggingen beheren? Beheerders en de kerngroep. */
function dg_cf_beheerder(): bool {
	if ( current_user_can( 'manage_options' ) ) {
		return true;
	}
	$groepen = (array) get_user_meta( get_current_user_id(), 'dg_groepen', true );
	return is_user_logged_in() && in_array( '/netwerk/kerngroep', $groepen, true );
}

/** De organisatie waarvoor de gebruiker mag toezeggen: alias en naam, of null. */
function dg_cf_organisatie( int $uid ): ?array {
	$alias = sanitize_key( (string) get_user_meta( $uid, 'dg_organisatie_alias', true ) );
	if ( '' === $alias ) {
		return null;
	}
	$naam = trim( (string) get_user_meta( $uid, 'dg_organisatie', true ) );
	return array( 'alias' => $alias, 'naam' => $naam ?: $alias );
}

/** Alle toezeggingen van een campagne die meetellen (intentie en bevestigd). */
function dg_cf_toezeggingen( string $campagne, bool $ook_oud = false ): array {
	global $wpdb;
	$tabel = dg_cf_tabel();
	$waar  = $ook_oud ? '' : " AND status IN ('intentie','bevestigd')";
	// phpcs:ignore WordPress.DB.PreparedSQL.InterpolatedNotPrepared -- tabelnaam en vaste voorwaarde.
	return (array) $wpdb->get_results( $wpdb->prepare( "SELECT * FROM $tabel WHERE campagne = %s$waar ORDER BY status = 'bevestigd' DESC, organisatie_naam", $campagne ), ARRAY_A );
}

function dg_cf_van( string $campagne, string $organisatie ): ?array {
	global $wpdb;
	$tabel = dg_cf_tabel();
	// phpcs:ignore WordPress.DB.PreparedSQL.InterpolatedNotPrepared
	$rij = $wpdb->get_row( $wpdb->prepare( "SELECT * FROM $tabel WHERE campagne = %s AND organisatie = %s AND status IN ('intentie','bevestigd') ORDER BY id DESC LIMIT 1", $campagne, $organisatie ), ARRAY_A );
	return $rij ?: null;
}

function dg_cf_euro( int $bedrag ): string {
	return '€ ' . number_format( $bedrag, 0, ',', '.' );
}

/** Een notitie in CiviCRM bij de gemeente, als CiviCRM aanstaat. Fouten worden genegeerd. */
function dg_cf_civicrm( array $rij, string $wat ): void {
	if ( ! function_exists( 'civicrm_initialize' ) || ! civicrm_initialize() || ! class_exists( '\Civi\Api4\Activity' ) ) {
		return;
	}
	try {
		$soort = 'dg_toezegging';
		if ( ! \Civi\Api4\OptionValue::get( false )->addWhere( 'option_group_id:name', '=', 'activity_type' )->addWhere( 'name', '=', $soort )->execute()->first() ) {
			\Civi\Api4\OptionValue::create( false )
				->addValue( 'option_group_id:name', 'activity_type' )
				->addValue( 'name', $soort )
				->addValue( 'label', 'Toezegging crowdfunding' )
				->addValue( 'is_reserved', true )
				->execute();
		}
		$org = \Civi\Api4\Contact::get( false )
			->addWhere( 'contact_type', '=', 'Organization' )
			->addWhere( 'organization_name', '=', $rij['organisatie_naam'] )
			->addWhere( 'is_deleted', '=', false )
			->addSelect( 'id' )->execute()->first();
		$org_id = $org ? (int) $org['id'] : (int) \Civi\Api4\Contact::create( false )
			->addValue( 'contact_type', 'Organization' )
			->addValue( 'organization_name', $rij['organisatie_naam'] )
			->execute()->first()['id'];
		$door = function_exists( 'dg_civicrm_contact' ) ? dg_civicrm_contact( get_current_user_id() ) : null;
		\Civi\Api4\Activity::create( false )
			->addValue( 'activity_type_id:name', $soort )
			->addValue( 'subject', sprintf( '%s: %s %s', $rij['campagne'], $wat, dg_cf_euro( (int) $rij['bedrag'] ) ) )
			->addValue( 'details', sprintf( 'Status: %s. Tekenbevoegd: %s (%s). Alleen bij het streefbedrag: %s.%s', $rij['status'], $rij['bevoegde_naam'], $rij['bevoegde_functie'], $rij['voorwaarde'] ? 'ja' : 'nee', $rij['kenmerk'] ? ' Kenmerk overeenkomst: ' . $rij['kenmerk'] . '.' : '' ) )
			->addValue( 'source_contact_id', $door ?: $org_id )
			->addValue( 'target_contact_id', array( $org_id ) )
			->addValue( 'status_id:name', 'Completed' )
			->execute();
	} catch ( Throwable $fout ) {
		unset( $fout );
	}
}

/** Een mail aan de kerngroep (beheeradres van de site) en aan wie de intentie doorgaf. */
function dg_cf_mail( array $rij, string $wat ): void {
	$onderwerp = sprintf( '[Crowdfunding] %s: %s %s', $rij['organisatie_naam'], $wat, $rij['campagne'] );
	$tekst     = sprintf(
		"%s heeft voor de campagne \"%s\" %s: %s.\n\nTekenbevoegd: %s (%s)\nAlleen als het streefbedrag gehaald wordt: %s\n\nBeheer: %s\n",
		$rij['organisatie_naam'],
		$rij['campagne'],
		$wat,
		dg_cf_euro( (int) $rij['bedrag'] ),
		$rij['bevoegde_naam'],
		$rij['bevoegde_functie'],
		$rij['voorwaarde'] ? 'ja' : 'nee',
		admin_url( 'admin.php?page=dg-toezeggingen' )
	);
	wp_mail( get_option( 'admin_email' ), $onderwerp, $tekst );
	$gebruiker = get_userdata( (int) $rij['gebruiker'] );
	if ( $gebruiker && 'intentie doorgegeven' === $wat ) {
		wp_mail(
			$gebruiker->user_email,
			'Je intentie voor ' . $rij['campagne'] . ' is ontvangen',
			"Dank je. De intentie van {$rij['organisatie_naam']} staat op de site. Een intentie is nog niet bindend: de kerngroep neemt contact op over de samenwerkingsovereenkomst. Pas als die is getekend, telt de bijdrage als bevestigd.\n"
		);
	}
}

// ---------------------------------------------------------------------------
// [dg_campagne id="…" doel="50000" einddatum="2027-03-01"]
// ---------------------------------------------------------------------------

add_shortcode( 'dg_campagne', 'dg_campagne' );

function dg_campagne( $attr ): string {
	$id        = sanitize_key( (string) ( $attr['id'] ?? '' ) );
	$doel      = (int) ( $attr['doel'] ?? 0 );
	$einde     = (string) ( $attr['einddatum'] ?? '' );
	$open      = '' === $einde || strtotime( $einde . ' 23:59:59' ) >= time();
	if ( '' === $id || $doel <= 0 ) {
		return '';
	}
	$rijen     = dg_cf_toezeggingen( $id );
	$bevestigd = array_sum( array_map( static fn( $r ) => 'bevestigd' === $r['status'] ? (int) $r['bedrag'] : 0, $rijen ) );
	$intentie  = array_sum( array_map( static fn( $r ) => 'intentie' === $r['status'] ? (int) $r['bedrag'] : 0, $rijen ) );
	$p_b       = min( 100, (int) round( 100 * $bevestigd / $doel ) );
	$p_i       = min( 100 - $p_b, (int) round( 100 * $intentie / $doel ) );

	$html  = '<div class="dg-cf" id="campagne-' . esc_attr( $id ) . '">';
	$html .= dg_cf_melding( $id );
	$lid   = is_user_logged_in();
	if ( $lid ) {
		$html .= '<span class="dg-balk" role="img" aria-label="' . esc_attr( "$p_b procent bevestigd, $p_i procent intentie" ) . '"><span class="dg-balk-vul" style="width:' . $p_b . '%"></span><span class="dg-balk-intentie" style="width:' . $p_i . '%"></span></span>';
	} else {
		$p_t   = min( 100, (int) round( 100 * ( $bevestigd + $intentie ) / $doel ) );
		$html .= '<span class="dg-balk" role="img" aria-label="' . esc_attr( "$p_t procent toegezegd" ) . '"><span class="dg-balk-vul" style="width:' . $p_t . '%"></span></span>';
	}
	$tot   = '';
	if ( $einde ) {
		$tot = $open ? ' · tot ' . esc_html( wp_date( 'j F Y', strtotime( $einde ) ) ) : ' · gesloten';
	}
	if ( ! $lid ) {
		// Bezoekers zien alleen het totaal en het doel; welke gemeenten meedoen, zien leden.
		$html .= '<span class="dg-bedragen"><strong>' . esc_html( dg_cf_euro( $bevestigd + $intentie ) ) . '</strong> toegezegd'
			. ' · doel ' . esc_html( dg_cf_euro( $doel ) ) . $tot . '</span>';
		$html .= dg_cf_formulier( $id, $open );
		return $html . '</div>';
	}
	$html .= '<span class="dg-bedragen"><strong>' . esc_html( dg_cf_euro( $bevestigd ) ) . '</strong> bevestigd';
	$html .= $intentie ? ' · ' . esc_html( dg_cf_euro( $intentie ) ) . ' intentie' : '';
	$html .= ' · doel ' . esc_html( dg_cf_euro( $doel ) ) . $tot . '</span>';
	if ( $rijen ) {
		$html .= '<ul class="dg-cf-gemeenten" aria-label="Gemeenten die meedoen">';
		foreach ( $rijen as $r ) {
			$html .= '<li><span>' . esc_html( $r['organisatie_naam'] ) . '</span>'
				. ( 'bevestigd' === $r['status'] ? '<span class="dg-db-status dg-db-status-actief">Bevestigd · ' . esc_html( dg_cf_euro( (int) $r['bedrag'] ) ) . '</span>' : '<span class="dg-db-status dg-db-status-in-verkenning">Intentie</span>' )
				. '</li>';
		}
		$html .= '</ul>';
	}
	$html .= dg_cf_formulier( $id, $open );
	return $html . '</div>';
}

/** De melding na het versturen (?dg_cf=…). */
function dg_cf_melding( string $id ): string {
	// phpcs:ignore WordPress.Security.NonceVerification.Recommended -- alleen een melding tonen.
	if ( ( $_GET['dg_cf_campagne'] ?? '' ) !== $id ) {
		return '';
	}
	$meldingen = array(
		'ok'          => 'Dank je. Je intentie staat erbij. De kerngroep neemt contact op over de samenwerkingsovereenkomst.',
		'ingetrokken' => 'De intentie is ingetrokken.',
		'fout'        => 'Dat lukte niet. Controleer het bedrag en de tekenbevoegde, en probeer het opnieuw.',
		'vast'        => 'Deze toezegging is al bevestigd met een samenwerkingsovereenkomst. Wijzigen kan via de kerngroep.',
	);
	$soort = sanitize_key( wp_unslash( $_GET['dg_cf'] ?? '' ) ); // phpcs:ignore WordPress.Security.NonceVerification.Recommended
	return isset( $meldingen[ $soort ] ) ? '<p class="dg-cf-melding dg-cf-melding-' . esc_attr( $soort ) . '" role="status">' . esc_html( $meldingen[ $soort ] ) . '</p>' : '';
}

/** Het formulier, of uitleg waarom je (nog) niet kunt toezeggen. */
function dg_cf_formulier( string $id, bool $open ): string {
	if ( ! $open ) {
		return '';
	}
	if ( ! is_user_logged_in() ) {
		return '<p class="dg-cf-uitleg"><a href="' . esc_url( wp_login_url( get_permalink() . '#campagne-' . $id ) ) . '">Log in</a> om namens je gemeente een intentie door te geven.</p>';
	}
	$uid = get_current_user_id();
	$org = dg_cf_organisatie( $uid );
	if ( ! $org ) {
		return '<p class="dg-cf-uitleg">Je account is niet gekoppeld aan een gemeente. Dat gebeurt vanzelf als je je aanmeldt met het e-mailadres van je werk en je gemeente bij de community hoort. Vraag het anders aan de kerngroep.</p>';
	}
	$nu = dg_cf_van( $id, $org['alias'] );
	if ( $nu && 'bevestigd' === $nu['status'] ) {
		return '<p class="dg-cf-uitleg">' . esc_html( $org['naam'] ) . ' doet mee: bevestigd met een samenwerkingsovereenkomst.</p>';
	}
	$actie = esc_url( admin_url( 'admin-post.php' ) );
	$f     = '<details class="dg-cf-form"><summary>' . ( $nu ? 'Intentie van ' . esc_html( $org['naam'] ) . ' wijzigen' : 'Intentie doorgeven namens ' . esc_html( $org['naam'] ) ) . '</summary>'
		. '<form method="post" action="' . $actie . '">'
		. wp_nonce_field( 'dg_cf_' . $id, '_wpnonce', true, false )
		. '<input type="hidden" name="action" value="dg_toezegging"><input type="hidden" name="campagne" value="' . esc_attr( $id ) . '">'
		. '<label>Bedrag in euro\'s<input type="number" name="bedrag" min="500" step="500" required value="' . esc_attr( $nu['bedrag'] ?? '' ) . '"></label>'
		. '<label>Wie is bevoegd om de overeenkomst te tekenen?<input type="text" name="bevoegde_naam" maxlength="190" required placeholder="Naam" value="' . esc_attr( $nu['bevoegde_naam'] ?? '' ) . '"></label>'
		. '<label>Functie<input type="text" name="bevoegde_functie" maxlength="190" required placeholder="Bijvoorbeeld wethouder of budgethouder" value="' . esc_attr( $nu['bevoegde_functie'] ?? '' ) . '"></label>'
		. '<label class="dg-cf-vink"><input type="checkbox" name="voorwaarde" value="1"' . checked( ! empty( $nu['voorwaarde'] ), true, false ) . '> Alleen als het streefbedrag gehaald wordt</label>'
		. '<label class="dg-cf-vink"><input type="checkbox" name="akkoord" value="1" required> Ik geef deze intentie door met medeweten van de tekenbevoegde. Ik begrijp dat ze niet bindend is: deelname wordt definitief met een getekende samenwerkingsovereenkomst.</label>'
		. '<button type="submit" class="dg-db-knop">' . ( $nu ? 'Wijziging opslaan' : 'Intentie doorgeven' ) . '</button>'
		. ( $nu ? ' <button type="submit" name="intrekken" value="1" class="dg-db-knop dg-db-knop-rand" formnovalidate>Intentie intrekken</button>' : '' )
		. '</form></details>';
	return $f;
}

// Het formulier verwerken.
add_action(
	'admin_post_dg_toezegging',
	static function () {
		$id    = sanitize_key( wp_unslash( $_POST['campagne'] ?? '' ) );
		$terug = wp_get_referer() ?: home_url( '/' );
		$terug = remove_query_arg( array( 'dg_cf', 'dg_cf_campagne' ), $terug );
		$naar  = static function ( string $soort ) use ( $id, $terug ): void {
			wp_safe_redirect( add_query_arg( array( 'dg_cf' => $soort, 'dg_cf_campagne' => $id ), strtok( $terug, '#' ) ) . '#campagne-' . $id );
			exit;
		};
		if ( ! $id || ! check_admin_referer( 'dg_cf_' . $id ) ) {
			$naar( 'fout' );
		}
		$uid = get_current_user_id();
		$org = dg_cf_organisatie( $uid );
		if ( ! $org ) {
			$naar( 'fout' );
		}
		global $wpdb;
		$nu    = dg_cf_van( $id, $org['alias'] );
		$tijd  = current_time( 'mysql', true );
		if ( $nu && 'bevestigd' === $nu['status'] ) {
			$naar( 'vast' );
		}
		if ( ! empty( $_POST['intrekken'] ) ) {
			if ( $nu ) {
				$wpdb->update( dg_cf_tabel(), array( 'status' => 'ingetrokken', 'gewijzigd' => $tijd, 'gebruiker' => $uid ), array( 'id' => $nu['id'] ) );
				$nu['status'] = 'ingetrokken';
				dg_cf_civicrm( $nu, 'intentie ingetrokken' );
				dg_cf_mail( $nu, 'intentie ingetrokken' );
			}
			$naar( 'ingetrokken' );
		}
		$bedrag = (int) ( $_POST['bedrag'] ?? 0 );
		$naam   = sanitize_text_field( wp_unslash( $_POST['bevoegde_naam'] ?? '' ) );
		$functie = sanitize_text_field( wp_unslash( $_POST['bevoegde_functie'] ?? '' ) );
		if ( $bedrag < 500 || $bedrag > 10000000 || '' === $naam || '' === $functie || empty( $_POST['akkoord'] ) ) {
			$naar( 'fout' );
		}
		$rij = array(
			'campagne'         => $id,
			'organisatie'      => $org['alias'],
			'organisatie_naam' => $nu['organisatie_naam'] ?? $org['naam'],
			'bedrag'           => $bedrag,
			'voorwaarde'       => empty( $_POST['voorwaarde'] ) ? 0 : 1,
			'bevoegde_naam'    => $naam,
			'bevoegde_functie' => $functie,
			'status'           => 'intentie',
			'gebruiker'        => $uid,
			'gewijzigd'        => $tijd,
		);
		if ( $nu ) {
			$wpdb->update( dg_cf_tabel(), $rij, array( 'id' => $nu['id'] ) );
		} else {
			$rij['aangemaakt'] = $tijd;
			$wpdb->insert( dg_cf_tabel(), $rij );
		}
		$rij['kenmerk'] = '';
		dg_cf_civicrm( $rij, $nu ? 'intentie gewijzigd' : 'intentie doorgegeven' );
		dg_cf_mail( $rij, $nu ? 'intentie gewijzigd' : 'intentie doorgegeven' );
		$naar( 'ok' );
	}
);

// ---------------------------------------------------------------------------
// Beheer: Toezeggingen (beheerders en de kerngroep)
// ---------------------------------------------------------------------------

add_action(
	'admin_menu',
	static function () {
		if ( dg_cf_beheerder() ) {
			add_menu_page( 'Toezeggingen', 'Toezeggingen', 'read', 'dg-toezeggingen', 'dg_cf_beheer', 'dashicons-groups', 26 );
		}
	}
);

function dg_cf_beheer(): void {
	if ( ! dg_cf_beheerder() ) {
		wp_die( 'Alleen voor de kerngroep.' );
	}
	global $wpdb;
	$tabel = dg_cf_tabel();
	$rijen = (array) $wpdb->get_results( "SELECT * FROM $tabel ORDER BY campagne, CASE status WHEN 'intentie' THEN 0 WHEN 'bevestigd' THEN 1 WHEN 'ingetrokken' THEN 2 ELSE 3 END, organisatie_naam", ARRAY_A ); // phpcs:ignore WordPress.DB.PreparedSQL.InterpolatedNotPrepared
	echo '<div class="wrap"><h1>Toezeggingen crowdfunding</h1>';
	echo '<p>Een intentie zet je op <strong>bevestigd</strong> als de samenwerkingsovereenkomst getekend binnen is; vul dan het kenmerk van de overeenkomst in. De naam van de gemeente kun je hier verbeteren; zo staat hij op de site. Zet de getekende overeenkomst zelf niet hier, maar in het archief.</p>';
	if ( isset( $_GET['bijgewerkt'] ) ) { // phpcs:ignore WordPress.Security.NonceVerification.Recommended
		echo '<div class="notice notice-success"><p>Bijgewerkt.</p></div>';
	}
	if ( ! $rijen ) {
		echo '<p>Nog geen toezeggingen.</p></div>';
		return;
	}
	echo '<table class="widefat striped"><thead><tr><th>Campagne</th><th>Gemeente</th><th>Bedrag</th><th>Voorwaarde</th><th>Tekenbevoegd</th><th>Status</th><th>Gewijzigd</th><th>Actie</th></tr></thead><tbody>';
	foreach ( $rijen as $r ) {
		echo '<tr><td>' . esc_html( $r['campagne'] ) . '</td>';
		echo '<td><form method="post" action="' . esc_url( admin_url( 'admin-post.php' ) ) . '">' . wp_nonce_field( 'dg_cf_beheer_' . $r['id'], '_wpnonce', true, false )
			. '<input type="hidden" name="action" value="dg_toezegging_beheer"><input type="hidden" name="id" value="' . (int) $r['id'] . '">'
			. '<input type="text" name="organisatie_naam" value="' . esc_attr( $r['organisatie_naam'] ) . '" class="regular-text" style="width:14em"><br><small>' . esc_html( $r['organisatie'] ) . '</small></td>';
		echo '<td>' . esc_html( dg_cf_euro( (int) $r['bedrag'] ) ) . '</td><td>' . ( $r['voorwaarde'] ? 'alleen bij streefbedrag' : '—' ) . '</td>';
		echo '<td>' . esc_html( $r['bevoegde_naam'] ) . '<br><small>' . esc_html( $r['bevoegde_functie'] ) . '</small></td>';
		echo '<td><strong>' . esc_html( DG_CF_STATUSSEN[ $r['status'] ] ?? $r['status'] ) . '</strong>' . ( $r['bevestigd_op'] ? '<br><small>' . esc_html( $r['bevestigd_op'] . ' · ' . $r['kenmerk'] ) . '</small>' : '' ) . '</td>';
		echo '<td>' . esc_html( get_date_from_gmt( $r['gewijzigd'], 'j M Y H:i' ) ) . '</td><td>';
		if ( 'intentie' === $r['status'] ) {
			echo '<input type="text" name="kenmerk" placeholder="Kenmerk overeenkomst" style="width:11em"> <input type="date" name="bevestigd_op" value="' . esc_attr( wp_date( 'Y-m-d' ) ) . '"> '
				. '<button class="button button-primary" name="doe" value="bevestigen">Bevestigen</button> <button class="button" name="doe" value="vervallen">Vervallen</button>';
		} elseif ( 'bevestigd' === $r['status'] ) {
			echo '<button class="button" name="doe" value="heropenen">Terug naar intentie</button>';
		}
		echo ' <button class="button-link" name="doe" value="naam">Naam opslaan</button></form></td></tr>';
	}
	echo '</tbody></table></div>';
}

add_action(
	'admin_post_dg_toezegging_beheer',
	static function () {
		$id = (int) ( $_POST['id'] ?? 0 );
		if ( ! dg_cf_beheerder() || ! check_admin_referer( 'dg_cf_beheer_' . $id ) ) {
			wp_die( 'Niet toegestaan.' );
		}
		global $wpdb;
		$tabel = dg_cf_tabel();
		$rij   = $wpdb->get_row( $wpdb->prepare( "SELECT * FROM $tabel WHERE id = %d", $id ), ARRAY_A ); // phpcs:ignore WordPress.DB.PreparedSQL.InterpolatedNotPrepared
		if ( ! $rij ) {
			wp_die( 'Niet gevonden.' );
		}
		$doe   = sanitize_key( wp_unslash( $_POST['doe'] ?? '' ) );
		$nieuw = array(
			'organisatie_naam' => sanitize_text_field( wp_unslash( $_POST['organisatie_naam'] ?? $rij['organisatie_naam'] ) ) ?: $rij['organisatie_naam'],
			'gewijzigd'        => current_time( 'mysql', true ),
		);
		$wat = '';
		if ( 'bevestigen' === $doe && 'intentie' === $rij['status'] ) {
			$nieuw += array(
				'status'       => 'bevestigd',
				'kenmerk'      => sanitize_text_field( wp_unslash( $_POST['kenmerk'] ?? '' ) ),
				'bevestigd_op' => preg_match( '/^\d{4}-\d{2}-\d{2}$/', (string) ( $_POST['bevestigd_op'] ?? '' ) ) ? sanitize_text_field( wp_unslash( $_POST['bevestigd_op'] ) ) : wp_date( 'Y-m-d' ),
			);
			$wat = 'bevestigd';
		} elseif ( 'vervallen' === $doe && 'intentie' === $rij['status'] ) {
			$nieuw['status'] = 'vervallen';
			$wat             = 'vervallen';
		} elseif ( 'heropenen' === $doe && 'bevestigd' === $rij['status'] ) {
			$nieuw += array( 'status' => 'intentie', 'bevestigd_op' => null, 'kenmerk' => '' );
			$wat    = 'terug naar intentie';
		}
		$wpdb->update( $tabel, $nieuw, array( 'id' => $id ) );
		if ( $wat ) {
			dg_cf_civicrm( array_merge( $rij, $nieuw ), $wat );
		}
		wp_safe_redirect( admin_url( 'admin.php?page=dg-toezeggingen&bijgewerkt=1' ) );
		exit;
	}
);

// wp dg toezeggingen [<campagne>]: overzicht voor beheerders.
if ( defined( 'WP_CLI' ) && WP_CLI ) {
	WP_CLI::add_command(
		'dg toezeggingen',
		static function ( $args ) {
			global $wpdb;
			$tabel = dg_cf_tabel();
			$rijen = isset( $args[0] ) ? dg_cf_toezeggingen( sanitize_key( $args[0] ), true ) : (array) $wpdb->get_results( "SELECT * FROM $tabel ORDER BY campagne, id", ARRAY_A ); // phpcs:ignore WordPress.DB.PreparedSQL.InterpolatedNotPrepared
			WP_CLI\Utils\format_items( 'table', $rijen, array( 'id', 'campagne', 'organisatie_naam', 'bedrag', 'voorwaarde', 'status', 'bevestigd_op', 'kenmerk' ) );
		}
	);
}
